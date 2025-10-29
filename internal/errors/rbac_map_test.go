package errors

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/cloud-nimbus/firedoor/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestMapK8sErrorToRbacError(t *testing.T) {
	resourceGroup := schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "rolebindings"}
	tests := []struct {
		name        string
		inputErr    error
		wantReason  v1alpha1.BreakglassConditionReason
		wantRetry   bool
		wantMessage string
	}{
		{
			name:        "forbidden error map to permanaent rbac error",
			inputErr:    apierrors.NewForbidden(resourceGroup, "mock-role", errors.New("denied")),
			wantReason:  v1alpha1.ReasonRBACForbidden,
			wantRetry:   false,
			wantMessage: "forbidden",
		},
		{
			name:        "unauthorized maps to forbidden reason",
			inputErr:    apierrors.NewUnauthorized("need auth"),
			wantReason:  v1alpha1.ReasonRBACForbidden,
			wantRetry:   false,
			wantMessage: "unauthorized",
		},
		{
			name:        "timeout retries with timeout reason",
			inputErr:    apierrors.NewTimeoutError("timed out", 0),
			wantReason:  v1alpha1.ReasonRBACTimeout,
			wantRetry:   true,
			wantMessage: "timeout",
		},
		{
			name:        "server timeout retries",
			inputErr:    apierrors.NewServerTimeout(resourceGroup, "update", 1),
			wantReason:  v1alpha1.ReasonRBACTimeout,
			wantRetry:   true,
			wantMessage: "server timeout",
		},
		{
			name:        "too many requests retries",
			inputErr:    apierrors.NewTooManyRequests("throttled", 1),
			wantReason:  v1alpha1.ReasonRBACTimeout,
			wantRetry:   true,
			wantMessage: "too many requests",
		},
		{
			name:        "conflict retries",
			inputErr:    apierrors.NewConflict(resourceGroup, "mock-role", errors.New("conflict")),
			wantReason:  v1alpha1.ReasonRBACTimeout,
			wantRetry:   true,
			wantMessage: "conflict",
		},
		{
			name:        "internal error retries",
			inputErr:    apierrors.NewInternalError(errors.New("boom")),
			wantReason:  v1alpha1.ReasonRBACTimeout,
			wantRetry:   true,
			wantMessage: "server error",
		},
		{
			name:        "service unavailable retries",
			inputErr:    apierrors.NewServiceUnavailable("downstream unavailable"),
			wantReason:  v1alpha1.ReasonRBACTimeout,
			wantRetry:   true,
			wantMessage: "server error",
		},
		{
			name:        "bad request maps to invalid request",
			inputErr:    apierrors.NewBadRequest("bad request"),
			wantReason:  v1alpha1.ReasonInvalidRequest,
			wantRetry:   false,
			wantMessage: "invalid request",
		},
		{
			name:        "already exists maps to invalid request",
			inputErr:    apierrors.NewAlreadyExists(resourceGroup, "mock-role"),
			wantReason:  v1alpha1.ReasonInvalidRequest,
			wantRetry:   false,
			wantMessage: "invalid request",
		},
		{
			name:        "resource expired maps to invalid request",
			inputErr:    apierrors.NewResourceExpired("watch closed"),
			wantReason:  v1alpha1.ReasonInvalidRequest,
			wantRetry:   false,
			wantMessage: "invalid request",
		},
		{
			name:        "notFound error is ignored",
			inputErr:    apierrors.NewNotFound(resourceGroup, "mock-role"),
			wantReason:  "",
			wantRetry:   false,
			wantMessage: "",
		},
		{
			name:        "network error treated as retryable",
			inputErr:    &net.DNSError{IsTemporary: true},
			wantReason:  v1alpha1.ReasonRBACTimeout,
			wantRetry:   true,
			wantMessage: "transient error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapK8sErrorToRBACError("delete", "mock-role", tt.inputErr)

			if got == nil {
				if tt.wantReason != "" {
					t.Fatalf("expected RBACError, got nil")
				}
				return
			}

			if got.Condition != tt.wantReason {
				t.Errorf("Condition = %v, want %v", got.Condition, tt.wantReason)
			}
			if got.Retryable != tt.wantRetry {
				t.Errorf("Retryable = %v, want %v", got.Retryable, tt.wantRetry)
			}
			if !strings.Contains(strings.ToLower(got.Error()), strings.ToLower(tt.wantMessage)) {
				t.Errorf("Error() = %v, want substring %v", got.Error(), tt.wantMessage)
			}
		})
	}
}
