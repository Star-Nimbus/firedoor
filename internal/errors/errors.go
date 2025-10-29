package errors

import (
	"net"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Error messages for controller operations
const (
	// Controller errors
	ErrCreateController = "unable to create controller"
	ErrStartManager     = "unable to start manager"
	ErrRunManager       = "problem running manager"

	// Health check errors
	ErrSetupHealthCheck = "unable to set up health check"
	ErrSetupReadyCheck  = "unable to set up ready check"

	// Configuration errors
	ErrLoadConfig = "failed to load configuration"

	// OpenTelemetry errors
	ErrSetupOTel    = "Failed to setup OpenTelemetry"
	ErrShutdownOTel = "Failed to shutdown tracer provider"

	// HTTP/TLS errors
	ErrDisableHTTP2 = "disabling http/2"
)

// Error types for different categories of errors
type ControllerError struct {
	Operation string
	Err       error
}

func (e *ControllerError) Error() string {
	return e.Operation + ": " + e.Err.Error()
}

func (e *ControllerError) Unwrap() error {
	return e.Err
}

// NewControllerError creates a new controller error
func NewControllerError(operation string, err error) *ControllerError {
	return &ControllerError{
		Operation: operation,
		Err:       err,
	}
}

// HealthCheckError represents health check related errors
type HealthCheckError struct {
	CheckType string
	Err       error
}

func (e *HealthCheckError) Error() string {
	return "unable to set up " + e.CheckType + " check: " + e.Err.Error()
}

func (e *HealthCheckError) Unwrap() error {
	return e.Err
}

// NewHealthCheckError creates a new health check error
func NewHealthCheckError(checkType string, err error) *HealthCheckError {
	return &HealthCheckError{
		CheckType: checkType,
		Err:       err,
	}
}

// ConfigError represents configuration related errors
type ConfigError struct {
	Operation string
	Err       error
}

func (e *ConfigError) Error() string {
	return e.Operation + ": " + e.Err.Error()
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new configuration error
func NewConfigError(operation string, err error) *ConfigError {
	return &ConfigError{
		Operation: operation,
		Err:       err,
	}
}

// OTelError represents OpenTelemetry related errors
type OTelError struct {
	Operation string
	Err       error
}

func (e *OTelError) Error() string {
	return e.Operation + ": " + e.Err.Error()
}

func (e *OTelError) Unwrap() error {
	return e.Err
}

// NewOTelError creates a new OpenTelemetry error
func NewOTelError(operation string, err error) *OTelError {
	return &OTelError{
		Operation: operation,
		Err:       err,
	}
}

// IsRetryableK8sError determines if a Kubernetes API error should be retried
func IsRetryableK8sError(err error) bool {
	if err == nil {
		return false
	}

	// Treat clearly permanent client-side errors as non-retryable.
	switch {
	case IsNotFoundError(err),
		IsAlreadyExistsError(err),
		apierrors.IsBadRequest(err),
		apierrors.IsInvalid(err),
		apierrors.IsMethodNotSupported(err),
		apierrors.IsNotAcceptable(err),
		apierrors.IsRequestEntityTooLargeError(err),
		apierrors.IsUnsupportedMediaType(err),
		apierrors.IsUnauthorized(err),
		apierrors.IsForbidden(err),
		apierrors.IsGone(err),
		apierrors.IsResourceExpired(err):
		return false
	}

	// Known transient server-side conditions.
	if apierrors.IsTimeout(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsTooManyRequests(err) ||
		apierrors.IsInternalError(err) ||
		apierrors.IsUnexpectedServerError(err) ||
		apierrors.IsServiceUnavailable(err) ||
		apierrors.IsConflict(err) ||
		apierrors.IsStoreReadError(err) {
		return true
	}

	// Network errors that are temporary or timeouts should be retried.
	if ne, ok := err.(net.Error); ok {
		if ne.Timeout() || ne.Temporary() {
			return true
		}
	}

	// Default to retrying unknown errors to avoid missing transient conditions.
	return true
}

// IsNotFoundError checks if an error is a "not found" error
func IsNotFoundError(err error) bool {
	return apierrors.IsNotFound(err)
}

// IsAlreadyExistsError checks if an error is an "already exists" error
func IsAlreadyExistsError(err error) bool {
	return apierrors.IsAlreadyExists(err)
}
