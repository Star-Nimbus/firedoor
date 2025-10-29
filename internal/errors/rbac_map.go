package errors

import (
	"fmt"

	"github.com/cloud-nimbus/firedoor/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// MapK8sErrorToRBACError maps a Kubernetes API error to an RBAC error
// It provides error model for the controller when it attempts to delete
func MapK8sErrorToRBACError(op, resource string, err error) *RBACError {
	if err == nil {
		return nil
	}

	if apierrors.IsNotFound(err) {
		// Resource already deleted.
		return nil
	}

	switch {
	case apierrors.IsForbidden(err):
		return NewPermanentRBACError(op, resource,
			v1alpha1.ReasonRBACForbidden,
			fmt.Errorf("forbidden: %w", err))
	case apierrors.IsUnauthorized(err):
		return NewPermanentRBACError(op, resource,
			v1alpha1.ReasonRBACForbidden,
			fmt.Errorf("unauthorized: %w", err))
	case apierrors.IsTimeout(err):
		return NewRetryableRBACError(op, resource,
			v1alpha1.ReasonRBACTimeout,
			fmt.Errorf("timeout: %w", err))
	case apierrors.IsServerTimeout(err):
		return NewRetryableRBACError(op, resource,
			v1alpha1.ReasonRBACTimeout,
			fmt.Errorf("server timeout: %w", err))
	case apierrors.IsTooManyRequests(err):
		return NewRetryableRBACError(op, resource,
			v1alpha1.ReasonRBACTimeout,
			fmt.Errorf("too many requests: %w", err))
	case apierrors.IsConflict(err):
		return NewRetryableRBACError(op, resource,
			v1alpha1.ReasonRBACTimeout,
			fmt.Errorf("conflict: %w", err))
	case apierrors.IsInternalError(err),
		apierrors.IsUnexpectedServerError(err),
		apierrors.IsServiceUnavailable(err),
		apierrors.IsStoreReadError(err):
		return NewRetryableRBACError(op, resource,
			v1alpha1.ReasonRBACTimeout,
			fmt.Errorf("server error: %w", err))
	case apierrors.IsBadRequest(err),
		apierrors.IsInvalid(err),
		apierrors.IsMethodNotSupported(err),
		apierrors.IsNotAcceptable(err),
		apierrors.IsRequestEntityTooLargeError(err),
		apierrors.IsUnsupportedMediaType(err),
		apierrors.IsAlreadyExists(err),
		apierrors.IsResourceExpired(err),
		apierrors.IsGone(err):
		return NewPermanentRBACError(op, resource,
			v1alpha1.ReasonInvalidRequest,
			fmt.Errorf("invalid request: %w", err))
	}

	if IsRetryableK8sError(err) {
		return NewRetryableRBACError(op, resource,
			v1alpha1.ReasonRBACTimeout,
			fmt.Errorf("transient error: %w", err))
	}

	return NewPermanentRBACError(op, resource,
		v1alpha1.ReasonInvalidRequest,
		fmt.Errorf("unhandled error: %w", err))
}
