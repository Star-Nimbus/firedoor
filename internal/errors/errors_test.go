package errors

import (
	"errors"
	"net"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestControllerError(t *testing.T) {
	originalErr := errors.New("test error")
	controllerErr := NewControllerError("test operation", originalErr)

	expected := "test operation: test error"
	if controllerErr.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, controllerErr.Error())
	}

	if controllerErr.Unwrap() != originalErr {
		t.Errorf("Expected unwrapped error to be the original error")
	}
}

func TestHealthCheckError(t *testing.T) {
	originalErr := errors.New("test error")
	healthErr := NewHealthCheckError("healthz", originalErr)

	expected := "unable to set up healthz check: test error"
	if healthErr.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, healthErr.Error())
	}

	if healthErr.Unwrap() != originalErr {
		t.Errorf("Expected unwrapped error to be the original error")
	}
}

func TestConfigError(t *testing.T) {
	originalErr := errors.New("test error")
	configErr := NewConfigError("load config", originalErr)

	expected := "load config: test error"
	if configErr.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, configErr.Error())
	}

	if configErr.Unwrap() != originalErr {
		t.Errorf("Expected unwrapped error to be the original error")
	}
}

func TestOTelError(t *testing.T) {
	originalErr := errors.New("test error")
	otelErr := NewOTelError("setup", originalErr)

	expected := "setup: test error"
	if otelErr.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, otelErr.Error())
	}

	if otelErr.Unwrap() != originalErr {
		t.Errorf("Expected unwrapped error to be the original error")
	}
}

func TestIsRetryableK8sError(t *testing.T) {
	gr := schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "rolebindings"}
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "not found", err: apierrors.NewNotFound(gr, "foo"), want: false},
		{name: "already exists", err: apierrors.NewAlreadyExists(gr, "foo"), want: false},
		{name: "forbidden", err: apierrors.NewForbidden(gr, "foo", errors.New("denied")), want: false},
		{name: "bad request", err: apierrors.NewBadRequest("bad request"), want: false},
		{name: "timeout", err: apierrors.NewTimeoutError("timed out", 0), want: true},
		{name: "server timeout", err: apierrors.NewServerTimeout(gr, "update", 1), want: true},
		{name: "too many requests", err: apierrors.NewTooManyRequests("throttled", 1), want: true},
		{name: "conflict", err: apierrors.NewConflict(gr, "foo", errors.New("conflict")), want: true},
		{name: "internal error", err: apierrors.NewInternalError(errors.New("boom")), want: true},
		{name: "service unavailable", err: apierrors.NewServiceUnavailable("unavailable"), want: true},
		{name: "network timeout", err: &net.DNSError{IsTimeout: true}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryableK8sError(tt.err); got != tt.want {
				t.Errorf("IsRetryableK8sError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	gr := schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "rolebindings"}
	if !IsNotFoundError(apierrors.NewNotFound(gr, "foo")) {
		t.Fatalf("expected IsNotFoundError to return true")
	}
	if IsNotFoundError(apierrors.NewForbidden(gr, "foo", errors.New("denied"))) {
		t.Fatalf("expected IsNotFoundError to return false")
	}
}

func TestIsAlreadyExistsError(t *testing.T) {
	gr := schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "rolebindings"}
	if !IsAlreadyExistsError(apierrors.NewAlreadyExists(gr, "foo")) {
		t.Fatalf("expected IsAlreadyExistsError to return true")
	}
	if IsAlreadyExistsError(apierrors.NewNotFound(gr, "foo")) {
		t.Fatalf("expected IsAlreadyExistsError to return false")
	}
}
