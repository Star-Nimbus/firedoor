package errors

import (
	"errors"
	"testing"
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

func TestErrorConstants(t *testing.T) {
	// Test that error constants are not empty
	if ErrCreateController == "" {
		t.Error("ErrCreateController should not be empty")
	}

	if ErrStartManager == "" {
		t.Error("ErrStartManager should not be empty")
	}

	if ErrRunManager == "" {
		t.Error("ErrRunManager should not be empty")
	}

	if ErrSetupHealthCheck == "" {
		t.Error("ErrSetupHealthCheck should not be empty")
	}

	if ErrSetupReadyCheck == "" {
		t.Error("ErrSetupReadyCheck should not be empty")
	}

	if ErrLoadConfig == "" {
		t.Error("ErrLoadConfig should not be empty")
	}

	if ErrSetupOTel == "" {
		t.Error("ErrSetupOTel should not be empty")
	}

	if ErrShutdownOTel == "" {
		t.Error("ErrShutdownOTel should not be empty")
	}

	if ErrDisableHTTP2 == "" {
		t.Error("ErrDisableHTTP2 should not be empty")
	}
}
