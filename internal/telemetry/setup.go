package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/cloud-nimbus/firedoor/internal/config"
	"github.com/cloud-nimbus/firedoor/internal/telemetry/metrics"
	"github.com/cloud-nimbus/firedoor/internal/telemetry/tracing"
)

func zapLevelFromString(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// Setup initializes all telemetry components (logging, tracing, metrics) based on the provided configuration.
// It returns a single shutdown function that gracefully terminates all telemetry components.
func Setup(ctx context.Context, cfg *config.Config, serviceName, serviceVersion, logLevel string) (func(), error) {
	var tp *sdktrace.TracerProvider

	// Configure logging: use Zap with the log level from CLI/config
	zapOpts := zap.Options{
		Development: logLevel == "debug",
		Level:       zapLevelFromString(logLevel),
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	// Configure tracing
	if cfg.OTel.Enabled {
		var err error
		tP, err := tracing.SetupTracing(
			ctx,
			cfg.OTel.Exporter,
			cfg.OTel.Endpoint,
			serviceName,
			serviceVersion,
			cfg.OTel.TLS.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to setup tracing: %w", err)
		}
		tP.ForceFlush(ctx) // ensure traces are flushed on shutdown
		tp = tP
		otel.SetTracerProvider(tp)
	}

	// Configure and initialize metrics
	if err := metrics.SetupPrometheus(ctrlmetrics.Registry); err != nil {
		return nil, fmt.Errorf("failed to setup prometheus: %w", err)
	}
	metrics.Init(cfg)

	// Create a single shutdown function for all components
	if tp == nil {
		return func() {}, nil // no-op
	}
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if tp != nil {
			if err := tp.Shutdown(shutdownCtx); err != nil {
				fmt.Printf("failed to shutdown OpenTelemetry tracer: %v\n", err)
			}
		}
	}, nil
}
