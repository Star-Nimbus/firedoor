package errors

import (
	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
)

// RBACError represents RBAC operation related errors
type RBACError struct {
	Operation string
	Resource  string
	Condition accessv1alpha1.BreakglassConditionReason
	Err       error
	Retryable bool
}

func (e *RBACError) Error() string {
	if e.Err == nil {
		return e.Operation + " " + e.Resource + ": nil"
	}
	return e.Operation + " " + e.Resource + ": " + e.Err.Error()
}

func (e *RBACError) Unwrap() error {
	return e.Err
}

// IsRetryable returns whether this error should trigger a retry
func (e *RBACError) IsRetryable() bool {
	return e.Retryable
}

// NewRBACError creates a new RBAC error
func NewRBACError(
	operation, resource string,
	condition accessv1alpha1.BreakglassConditionReason, err error, retryable bool) *RBACError {
	return &RBACError{
		Operation: operation,
		Resource:  resource,
		Condition: condition,
		Err:       err,
		Retryable: retryable,
	}
}

// NewRetryableRBACError creates a retryable RBACError.
func NewRetryableRBACError(
	operation, resource string,
	condition accessv1alpha1.BreakglassConditionReason,
	err error,
) *RBACError {
	return NewRBACError(operation, resource, condition, err, true)
}

// NewPermanentRBACError creates a non-retryable RBACError.
func NewPermanentRBACError(
	operation, resource string,
	condition accessv1alpha1.BreakglassConditionReason,
	err error,
) *RBACError {
	return NewRBACError(operation, resource, condition, err, false)
}
