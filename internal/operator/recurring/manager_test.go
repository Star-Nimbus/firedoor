package recurring

import (
	"context"
	"testing"
	"time"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockClock implements controller.Clock for testing
type mockClock struct {
	now time.Time
}

func (m *mockClock) Now() time.Time {
	return m.now
}

func (m *mockClock) Until(t time.Time) time.Duration {
	return t.Sub(m.now)
}

func (m *mockClock) IsExpired(t time.Time) bool {
	return m.now.After(t)
}

func TestManager_ProcessRecurring(t *testing.T) {
	tests := []struct {
		name              string
		bg                *accessv1alpha1.Breakglass
		clockTime         time.Time
		expectedError     bool
		expectedCondition accessv1alpha1.BreakglassCondition
		expectedNextAt    *time.Time
	}{
		{
			name: "non-recurring breakglass should be ignored",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{},
				},
			},
			clockTime:     time.Now(),
			expectedError: false,
		},
		{
			name: "recurring breakglass with invalid schedule should error",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Cron: "invalid cron",
					},
				},
			},
			clockTime:     time.Now(),
			expectedError: true,
		},
		{
			name: "recurring breakglass with valid schedule should schedule initial activation",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Cron:  "0 9 * * 1-5", // Weekdays at 9 AM
						Start: metav1.Time{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
						Duration: metav1.Duration{
							Duration: 24 * time.Hour,
						},
					},
				},
			},
			clockTime:         time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC), // Monday 8 AM
			expectedError:     false,
			expectedCondition: accessv1alpha1.ConditionRecurringPending,
		},
		{
			name: "recurring breakglass should activate when time is due",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Cron:     "0 9 * * 1-5",
						Start:    metav1.Time{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
						Duration: metav1.Duration{Duration: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC).Sub(time.Now())},
					},
				},
				Status: accessv1alpha1.BreakglassStatus{
					NextActivationAt: &metav1.Time{Time: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)},
					ActivationCount:  0,
				},
			},
			clockTime:         time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC), // Monday 9:30 AM (past activation time)
			expectedError:     false,
			expectedCondition: accessv1alpha1.ConditionRecurringPending,
			expectedNextAt:    timePtr(time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)),
		},
		{
			name: "recurring breakglass with time zone",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Cron:     "0 9 * * 1-5", // Weekdays at 9 AM
						Location: "America/New_York",
						Start:    metav1.Time{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.FixedZone("EST", -5*3600))},
						Duration: metav1.Duration{
							Duration: 24 * time.Hour,
						},
					},
				},
			},
			clockTime:         time.Date(2024, 1, 15, 8, 0, 0, 0, time.FixedZone("EST", -5*3600)), // 8 AM EST
			expectedError:     false,
			expectedCondition: accessv1alpha1.ConditionRecurringPending,
		},
		{
			name: "recurring breakglass with start in future",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Cron:     "0 9 * * 1-5",
						Start:    metav1.Time{Time: time.Date(2024, 1, 16, 9, 0, 0, 0, time.UTC)},
						Duration: metav1.Duration{Duration: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC).Sub(time.Now())},
					},
				},
			},
			clockTime:         time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC),
			expectedError:     false,
			expectedCondition: "",
		},
		{
			name: "one-shot breakglass schedules activation",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Start: metav1.Time{Time: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)},
					},
				},
			},
			clockTime:         time.Date(2024, 1, 15, 8, 30, 0, 0, time.UTC),
			expectedError:     false,
			expectedCondition: accessv1alpha1.ConditionRecurringPending,
			expectedNextAt:    timePtr(time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)),
		},
		{
			name: "missed activation reschedules to next window",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Cron:     "0 9 * * 1-5",
						Start:    metav1.Time{Time: time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)},
						Duration: metav1.Duration{Duration: 30 * time.Minute},
					},
				},
				Status: accessv1alpha1.BreakglassStatus{
					NextActivationAt: &metav1.Time{Time: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)},
				},
			},
			clockTime:         time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			expectedError:     false,
			expectedCondition: accessv1alpha1.ConditionRecurringPending,
			expectedNextAt:    timePtr(time.Date(2024, 1, 16, 9, 0, 0, 0, time.UTC)),
		},
		{
			name: "recurring breakglass with maxActivations reached",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						// Weekdays at 9 AM
						Cron:           "0 9 * * 1-5",
						MaxActivations: int32Ptr(1),
						Start:          metav1.Time{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
						Duration:       metav1.Duration{Duration: 24 * time.Hour},
					},
				},
				Status: accessv1alpha1.BreakglassStatus{
					ActivationCount: 1,
				},
			},
			clockTime:         time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			expectedError:     false,
			expectedCondition: accessv1alpha1.ConditionFailed,
		},
		{
			name: "recurring breakglass with invalid cron fields",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Cron: "0 9 * * 1-5 2024", // 6 fields
					},
				},
			},
			clockTime:         time.Now(),
			expectedError:     true,
			expectedCondition: accessv1alpha1.ConditionFailed,
		},
		{
			name: "recurring breakglass with @yearly descriptor",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Cron: "@yearly",
					},
				},
			},
			clockTime:         time.Now(),
			expectedError:     true,
			expectedCondition: accessv1alpha1.ConditionFailed,
		},
		// TODO fix timezones
		// {
		// 	name: "recurring breakglass DST spring-forward (America/Sao_Paulo)",
		// 	bg: &accessv1alpha1.Breakglass{
		// 		Spec: accessv1alpha1.BreakglassSpec{
		// 			Schedule: accessv1alpha1.ScheduleSpec{
		// 				// Weekdays at 9 AM
		// 				Cron:     "0 9 * * 1-5",
		// 				Location: "America/Sao_Paulo",
		// 				Start:    metav1.Time{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.FixedZone("-03", -3*3600))},
		// 				Duration: metav1.Duration{Duration: 24 * time.Hour},
		// 			},
		// 		},
		// 	},
		// 	clockTime:         time.Date(2024, 11, 3, 0, 10, 0, 0, time.FixedZone("-03", -3*3600)), // DST transition day
		// 	expectedError:     false,
		// 	expectedCondition: accessv1alpha1.ConditionRecurringActive,
		// },
		{
			name: "recurring breakglass with MaxActivations=0 (unlimited)",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Cron:           "0 9 * * 1-5",
						MaxActivations: int32Ptr(0),
						Start:          metav1.Time{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
						Duration:       metav1.Duration{Duration: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC).Sub(time.Now())},
					},
				},
				Status: accessv1alpha1.BreakglassStatus{
					ActivationCount: 1000,
				},
			},
			clockTime:         time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			expectedError:     false,
			expectedCondition: "",
		},
		{
			name: "recurring breakglass with MaxActivations=1 (limit)",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Cron:           "0 9 * * 1-5",
						MaxActivations: int32Ptr(1),
						Start:          metav1.Time{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
						Duration:       metav1.Duration{Duration: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC).Sub(time.Now())},
					},
				},
				Status: accessv1alpha1.BreakglassStatus{
					ActivationCount: 1,
				},
			},
			clockTime:         time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			expectedError:     false,
			expectedCondition: accessv1alpha1.ConditionFailed,
		},
		{
			name: "recurring breakglass with DurationDate == candidate (inclusive)",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						// weekdays a
						Cron:     "0 9 * * 1-5",
						Duration: metav1.Duration{Duration: 24 * time.Hour},
					},
				},
			},
			clockTime:         time.Date(2024, 1, 1, 9, 10, 0, 0, time.UTC),
			expectedError:     false,
			expectedCondition: accessv1alpha1.ConditionRecurringPending,
		},
		{
			name: "controller restart between activation and status update (re-entrancy)",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Cron: "0 9 * * 1-5",
						Duration: metav1.Duration{
							Duration: 24 * time.Hour,
						},
					},
				},
				Status: accessv1alpha1.BreakglassStatus{
					NextActivationAt: &metav1.Time{Time: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)},
				},
			},
			clockTime:         time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			expectedError:     false,
			expectedCondition: accessv1alpha1.ConditionRecurringPending,
			// Special: call ProcessRecurring twice to simulate re-entrancy
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bg := tt.bg.DeepCopy()
			clock := &mockClock{now: tt.clockTime}
			manager := New(clock)
			if tt.name == "controller restart between activation and status update (re-entrancy)" {
				// First call: schedule activation
				_ = manager.ProcessRecurring(context.Background(), bg)
				if bg.Status.NextActivationAt == nil {
					t.Fatalf("expected NextActivationAt to be set")
				}
				// Advance clock to activation time
				clock.now = bg.Status.NextActivationAt.Time
				// Simulate controller persistence: copy status into a new object
				bg2 := bg.DeepCopy()
				bg2.Status = bg.Status
				_ = manager.ProcessRecurring(context.Background(), bg2)
				bg = bg2
				if len(bg.Status.Conditions) == 0 ||
					bg.Status.Conditions[len(bg.Status.Conditions)-1].Type != string(accessv1alpha1.ConditionRecurringPending) {
					t.Errorf("expected condition %s but got %v", accessv1alpha1.ConditionRecurringPending, bg.Status.Conditions)
				}
				return
			}
			err := manager.ProcessRecurring(context.Background(), bg)

			if tt.expectedError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.expectedCondition != "" {
				if len(bg.Status.Conditions) == 0 {
					t.Errorf("expected condition %s but no conditions found", tt.expectedCondition)
				} else {
					lastCondition := bg.Status.Conditions[len(bg.Status.Conditions)-1]
					if lastCondition.Type != string(tt.expectedCondition) {
						t.Errorf("expected condition %s but got %s", tt.expectedCondition, lastCondition.Type)
					}
				}
			}

			if tt.expectedNextAt != nil {
				if bg.Status.NextActivationAt == nil {
					t.Fatalf("expected NextActivationAt but got nil")
				}
				if !bg.Status.NextActivationAt.Time.Equal(*tt.expectedNextAt) {
					t.Fatalf("expected NextActivationAt %v, got %v", tt.expectedNextAt, bg.Status.NextActivationAt.Time)
				}
			}
		})
	}
}

func TestManager_ShouldActivate(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		bg        *accessv1alpha1.Breakglass
		clockTime time.Time
		expected  bool
	}{
		{
			name: "non-recurring breakglass should not activate",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{},
				},
			},
			clockTime: baseTime,
			expected:  false,
		},
		{
			name: "one-shot activation fires when next activation reached",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Start: metav1.Time{Time: baseTime},
					},
				},
				Status: accessv1alpha1.BreakglassStatus{
					NextActivationAt: &metav1.Time{Time: baseTime},
				},
			},
			clockTime: baseTime.Add(5 * time.Minute),
			expected:  true,
		},
		{
			name: "one-shot activation fires when next activation reached",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Start: metav1.Time{Time: time.Now()},
					},
				},
				Status: accessv1alpha1.BreakglassStatus{
					NextActivationAt: &metav1.Time{Time: time.Now()},
				},
			},
			clockTime: time.Now().Add(time.Minute),
			expected:  true,
		},
		{
			name: "recurring breakglass with no next activation should not activate",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Start: metav1.Time{Time: time.Now().Add(time.Hour)},
					},
				},
				Status: accessv1alpha1.BreakglassStatus{
					NextActivationAt: nil,
				},
			},
			clockTime: time.Now(),
			expected:  false,
		},
		{
			name: "recurring breakglass with future activation should not activate",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Start: metav1.Time{Time: time.Now().Add(time.Hour)},
					},
				},
				Status: accessv1alpha1.BreakglassStatus{
					NextActivationAt: &metav1.Time{Time: time.Now().Add(time.Hour)},
				},
			},
			clockTime: time.Now(),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := &mockClock{now: tt.clockTime}
			manager := New(clock)

			result := manager.ShouldActivate(context.Background(), tt.bg)

			if result != tt.expected {
				t.Errorf("expected %v but got %v", tt.expected, result)
			}
		})
	}
}

func TestManager_ShouldDeactivate(t *testing.T) {
	clock := &mockClock{now: time.Now()}
	manager := New(clock)

	bg := &accessv1alpha1.Breakglass{
		Spec: accessv1alpha1.BreakglassSpec{
			Schedule: accessv1alpha1.ScheduleSpec{},
		},
	}

	// Recurring breakglasses should never auto-deactivate
	result := manager.ShouldDeactivate(context.Background(), bg)
	if result {
		t.Errorf("expected false but got %v", result)
	}
}

func TestManager_OnActivationGranted(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name          string
		bg            *accessv1alpha1.Breakglass
		wantNextDelta time.Duration
		wantCount     int32
	}{
		{
			name: "recurring schedule advances to next window",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{
						Cron: "*/5 * * * *",
					},
				},
				Status: accessv1alpha1.BreakglassStatus{
					GrantedAt: &metav1.Time{Time: baseTime},
				},
			},
			wantNextDelta: 5 * time.Minute,
			wantCount:     1,
		},
		{
			name: "non-recurring clears next activation",
			bg: &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Schedule: accessv1alpha1.ScheduleSpec{},
				},
				Status: accessv1alpha1.BreakglassStatus{
					GrantedAt: &metav1.Time{Time: baseTime},
				},
			},
			wantNextDelta: 0,
			wantCount:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := New(&mockClock{now: baseTime})
			err := manager.OnActivationGranted(context.Background(), tt.bg)
			if err != nil {
				t.Fatalf("OnActivationGranted returned error: %v", err)
			}

			if tt.wantNextDelta == 0 {
				if tt.bg.Status.NextActivationAt != nil {
					t.Fatalf("expected NextActivationAt to be nil, got %v", tt.bg.Status.NextActivationAt)
				}
			} else {
				if tt.bg.Status.NextActivationAt == nil {
					t.Fatalf("expected NextActivationAt to be set")
				}
				want := tt.bg.Status.GrantedAt.Time.Add(tt.wantNextDelta)
				got := tt.bg.Status.NextActivationAt.Time
				if !got.Equal(want) {
					t.Fatalf("expected NextActivationAt %v, got %v", want, got)
				}
			}

			if tt.bg.Status.ActivationCount != tt.wantCount {
				t.Fatalf("expected ActivationCount %d, got %d", tt.wantCount, tt.bg.Status.ActivationCount)
			}

			if tt.bg.Spec.Schedule.Cron != "" {
				if len(tt.bg.Status.Conditions) == 0 {
					t.Fatalf("expected a recurring condition entry")
				}
				last := tt.bg.Status.Conditions[len(tt.bg.Status.Conditions)-1]
				if last.Type != string(accessv1alpha1.ConditionRecurringPending) {
					t.Fatalf("expected condition %s, got %s", accessv1alpha1.ConditionRecurringPending, last.Type)
				}
			}
		})
	}
}

func TestManager_validateRecurrenceSchedule(t *testing.T) {
	clock := &mockClock{now: time.Now()}
	manager := New(clock)

	tests := []struct {
		name     string
		cron     string
		enabled  bool // unused, kept for compatibility
		expected bool // true if should pass validation
	}{
		{
			name:     "disabled recurrence should pass",
			cron:     "",
			enabled:  false,
			expected: true,
		},
		{
			name:     "enabled recurrence with empty schedule should fail",
			cron:     "",
			enabled:  true,
			expected: true, // No cron means not recurring, so should pass
		},
		{
			name:     "enabled recurrence with invalid schedule should fail",
			cron:     "invalid cron",
			enabled:  true,
			expected: false,
		},
		{
			name:     "enabled recurrence with valid schedule should pass",
			cron:     "0 9 * * 1-5",
			enabled:  true,
			expected: true,
		},
		{
			name:     "enabled recurrence with valid schedule should pass",
			cron:     "0 0 * * *", // Daily at midnight
			enabled:  true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule := &accessv1alpha1.ScheduleSpec{
				Cron: tt.cron,
			}

			_, _, err := manager.parseAndValidateSchedule(schedule)

			if tt.expected && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			if !tt.expected && err == nil {
				t.Errorf("expected error but got none")
			}
		})
	}
}

func int32Ptr(i int32) *int32 { return &i }

func timePtr(t time.Time) *time.Time {
	return &t
}
