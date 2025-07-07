package errors

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
