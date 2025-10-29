package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/controller/mocks"
	internalerrors "github.com/cloud-nimbus/firedoor/internal/errors"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// timeMatcher is a custom gomock matcher for time equality
type timeMatcher struct {
	expected time.Time
}

func (m *timeMatcher) Matches(x interface{}) bool {
	if t, ok := x.(time.Time); ok {
		return t.Equal(m.expected)
	}
	return false
}

func (m *timeMatcher) String() string {
	return "is equal to " + m.expected.String()
}

func timeEqual(t time.Time) gomock.Matcher {
	return &timeMatcher{expected: t}
}

func TestHandler_GrantAndActivate(t *testing.T) {
	mock_controller := gomock.NewController(t)
	defer mock_controller.Finish()

	// monday
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		duration    time.Duration
		expectCalls func(clock *mocks.MockClock, op *mocks.MockBreakglassOperator)
		wantExpires bool
		wantAfter   time.Duration
	}{
		{
			name:        "no duration, it does not expire",
			wantAfter:   0,
			wantExpires: false,
			duration:    0,
			expectCalls: func(clock *mocks.MockClock, op *mocks.MockBreakglassOperator) {
				op.EXPECT().GrantAccess(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				clock.EXPECT().Now().Return(now)
				clock.EXPECT().Until(gomock.Any()).Times(0)
			},
		},
		{
			name:        "has duration, it expires",
			duration:    time.Hour,
			wantAfter:   time.Hour - time.Minute,
			wantExpires: false,
			expectCalls: func(clock *mocks.MockClock, op *mocks.MockBreakglassOperator) {
				op.EXPECT().GrantAccess(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				clock.EXPECT().Now().Return(now)
				clock.EXPECT().Until(gomock.Any()).Return(time.Hour - time.Minute)
			},
		},
		{
			name:        "has duration, it expires soon, requeue at least 30s",
			duration:    time.Minute,
			wantExpires: false,
			wantAfter:   30 * time.Second,
			expectCalls: func(clock *mocks.MockClock, op *mocks.MockBreakglassOperator) {
				op.EXPECT().GrantAccess(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				clock.EXPECT().Now().Return(now)
				clock.EXPECT().Until(gomock.Any()).Return(10 * time.Second)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClock := mocks.NewMockClock(mock_controller)
			mockOperator := mocks.NewMockBreakglassOperator(mock_controller)
			ctx := context.TODO()

			tt.expectCalls(mockClock, mockOperator)

			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-breakglass",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Approval: &accessv1alpha1.ApprovalSpec{
						Required: false,
					},
					Schedule: accessv1alpha1.ScheduleSpec{
						Duration: metav1.Duration{
							Duration: tt.duration,
						},
					},
				},
			}

			scheme := runtime.NewScheme()
			_ = accessv1alpha1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&accessv1alpha1.Breakglass{}).
				WithObjects(bg).
				Build()
			handler := &Handler{
				Client:   fakeClient,
				Clock:    mockClock,
				Operator: mockOperator,
			}

			// Call the method under test
			got, _ := handler.GrantAndActivate(ctx, bg)
			// Re-fetch from fake client to observe .Status changes
			fresh := &accessv1alpha1.Breakglass{}
			if err := fakeClient.Get(ctx, client.ObjectKeyFromObject(bg), fresh); err != nil {
				t.Fatalf("failed to refetch breakglass: %v", err)
			}
			bg = fresh

			if bg.Status.GrantedAt == nil {
				t.Errorf("GrantAndActivate() GrantedAt not set")
			} else if !bg.Status.GrantedAt.Time.Equal(now) {
				t.Errorf("GrantAndActivate() GrantedAt = %v, want %v", bg.Status.GrantedAt.Time, now)
			}

			switch {
			case tt.wantExpires && bg.Status.ExpiresAt == nil:
				t.Errorf("GrantAndActivate() ExpiresAt = nil, want not nil")
			case !tt.wantExpires && bg.Status.ExpiresAt != nil:
				t.Errorf("GrantAndActivate() ExpiresAt = %v, want nil", bg.Status.ExpiresAt.Time)
			}

			gotAfter := got.RequeueAfter
			if gotAfter != tt.wantAfter {
				t.Errorf("GrantAndActivate() RequeueAfter = %v, want %v", gotAfter, tt.wantAfter)
			}

			if tt.wantAfter > 0 && gotAfter < tt.wantAfter {
				t.Errorf("GrantAndActivate() RequeueAfter = %v, want at least %v", gotAfter, tt.wantAfter)
			}

		})
	}
}

func TestHandler_RevokeAndExpire(t *testing.T) {
	mock_controller := gomock.NewController(t)
	defer mock_controller.Finish()

	ctx := context.TODO()
	scheme := runtime.NewScheme()
	_ = accessv1alpha1.AddToScheme(scheme)

	tests := []struct {
		name        string
		revokeErr   error
		wantCond    accessv1alpha1.BreakglassCondition
		wantReason  accessv1alpha1.BreakglassConditionReason
		wantRequeue time.Duration
		wantErr     bool
		prepare     func(bg *accessv1alpha1.Breakglass, clock *mocks.MockClock)
	}{
		{
			name:        "successful revoke, no requeue",
			revokeErr:   nil,
			wantCond:    accessv1alpha1.ConditionExpired,
			wantReason:  accessv1alpha1.ReasonAccessExpired,
			wantRequeue: 0,
			wantErr:     false,
			prepare: func(bg *accessv1alpha1.Breakglass, clock *mocks.MockClock) {
				granted := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
				expires := granted.Add(30 * time.Minute)
				bg.Spec.Schedule.Duration = metav1.Duration{Duration: 30 * time.Minute}
				bg.Status.GrantedAt = &metav1.Time{Time: granted}
				clock.EXPECT().Now().Return(expires)
			},
		},
		// TODO:Consider making the a conditional type of error to distinguish retryable vs permanent errors
		{
			name:       "retryable RBAC revoke error, requeue with backoff",
			revokeErr:  internalerrors.NewRetryableRBACError("revoke", "mock-role", accessv1alpha1.ReasonRBACTimeout, errors.New("simulated RBAC revoke error")),
			wantCond:   "",
			wantReason: "",
			wantErr:    false,
		},
		{
			name:        "recurring revoke transitions to pending",
			revokeErr:   nil,
			wantCond:    accessv1alpha1.ConditionRecurringPending,
			wantReason:  accessv1alpha1.ReasonRecurringScheduled,
			wantRequeue: 45 * time.Minute,
			wantErr:     false,
			prepare: func(bg *accessv1alpha1.Breakglass, clock *mocks.MockClock) {
				next := time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC)
				bg.Spec.Schedule = accessv1alpha1.ScheduleSpec{
					Cron:     "0 10 * * *",
					Duration: metav1.Duration{Duration: 15 * time.Minute},
				}
				bg.Status.NextActivationAt = &metav1.Time{Time: next}
				clock.EXPECT().Until(timeEqual(next)).Return(45 * time.Minute)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOperator := mocks.NewMockBreakglassOperator(mock_controller)
			mockClock := mocks.NewMockClock(mock_controller)

			bg := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-breakglass",
					Namespace: "default",
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Approval: &accessv1alpha1.ApprovalSpec{
						Required: false,
					},
				},
			}
			scheme := runtime.NewScheme()
			_ = accessv1alpha1.AddToScheme(scheme)

			if tt.prepare != nil {
				tt.prepare(bg, mockClock)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&accessv1alpha1.Breakglass{}).
				WithObjects(bg).
				Build()
			handler := &Handler{
				Client:   fakeClient,
				Clock:    mockClock,
				Operator: mockOperator,
			}
			mockOperator.EXPECT().RevokeAccess(gomock.Any(), gomock.Any()).Return(tt.revokeErr).AnyTimes()

			got, err := handler.RevokeAndExpire(ctx, bg)
			if err != nil != tt.wantErr {
				t.Fatalf("RevokeAndExpire() error %v, wantErr %v", err, tt.wantErr)
			}

			fresh := &accessv1alpha1.Breakglass{}
			if getErr := fakeClient.Get(ctx, client.ObjectKeyFromObject(bg), fresh); getErr != nil {
				t.Fatalf("failed to refetch breakglass: %v", getErr)
			}

			if tt.wantCond != "" {
				found := false
				for _, cond := range fresh.Status.Conditions {
					if cond.Type == string(tt.wantCond) {
						found = true
						if cond.Reason != string(tt.wantReason) {
							t.Errorf("Condition reason = %v, want %v", cond.Reason, tt.wantReason)
						}
					}
				}
				if !found {
					t.Errorf("Condition type = %v not found", tt.wantCond)
				}
			}

			if got.RequeueAfter != tt.wantRequeue {
				t.Errorf("RevokeAndExpire() RequeueAfter = %v, want %v", got.RequeueAfter, tt.wantRequeue)
			}

			if tt.wantCond == accessv1alpha1.ConditionExpired {
				if fresh.Status.ExpiresAt == nil {
					t.Fatalf("expected ExpiresAt to be set for expired breakglass")
				}
				expectedExpiry := fresh.Status.GrantedAt.Time.Add(fresh.Spec.Schedule.Duration.Duration)
				if !fresh.Status.ExpiresAt.Time.Equal(expectedExpiry) {
					t.Errorf("ExpiresAt = %v, want %v", fresh.Status.ExpiresAt.Time, expectedExpiry)
				}
			}

			if tt.wantCond == accessv1alpha1.ConditionRecurringPending && fresh.Status.ExpiresAt != nil {
				t.Errorf("expected ExpiresAt to be cleared for recurring revoke, got %v", fresh.Status.ExpiresAt)
			}
		})
	}
}

func TestEmitAccessGrantedEvent(t *testing.T) {
	// Test that the function doesn't panic when recorder is nil
	bg := &accessv1alpha1.Breakglass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-breakglass",
			Namespace: "default",
		},
		Spec: accessv1alpha1.BreakglassSpec{
			ClusterRoles: []string{"admin", "view"},
			Policy: []accessv1alpha1.Policy{
				{
					Namespace: "default",
					Rules:     []rbacv1.PolicyRule{},
				},
				{
					Namespace: "kube-system",
					Rules:     []rbacv1.PolicyRule{},
				},
			},
		},
	}

	handler := &Handler{
		recorder: nil, // Test with nil recorder
	}

	// This should not panic
	handler.emitAccessGrantedEvent(bg)
}

func TestEmitAccessRevokedEvent(t *testing.T) {
	// Test that the function doesn't panic when recorder is nil
	bg := &accessv1alpha1.Breakglass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-breakglass",
			Namespace: "default",
		},
	}

	handler := &Handler{
		recorder: nil, // Test with nil recorder
	}

	// This should not panic
	handler.emitAccessRevokedEvent(bg)
}

func TestEmitAccessGrantFailedEvent(t *testing.T) {
	// Test that the function doesn't panic when recorder is nil
	bg := &accessv1alpha1.Breakglass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-breakglass",
			Namespace: "default",
		},
	}

	handler := &Handler{
		recorder: nil, // Test with nil recorder
	}

	testErr := errors.New("test error")

	// This should not panic
	handler.emitAccessGrantFailedEvent(bg, testErr)
}

func TestEmitAccessRevokeFailedEvent(t *testing.T) {
	// Test that the function doesn't panic when recorder is nil
	bg := &accessv1alpha1.Breakglass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-breakglass",
			Namespace: "default",
		},
	}

	handler := &Handler{
		recorder: nil, // Test with nil recorder
	}

	testErr := errors.New("test error")

	// This should not panic
	handler.emitAccessRevokeFailedEvent(bg, testErr)
}
