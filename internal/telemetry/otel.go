package telemetry

import (
	"context"
	"fmt"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	ctrl "sigs.k8s.io/controller-runtime"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

var setupLog = ctrl.Log.WithName("setup")

// initTracing initializes OpenTelemetry tracing
func initTracing(ctx context.Context, serviceName, exporterType, endpoint string) (func(), error) {
	// Create resource with service information
	// TODO: Add version from build
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize exporter based on type
	var exporter sdktrace.SpanExporter
	switch exporterType {
	case "otlp":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithInsecure(), // Use WithTLSClientConfig for production
		}
		if endpoint != "" {
			opts = append(opts, otlptracehttp.WithEndpoint(endpoint))
		}
		exporter, err = otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
	case "stdout":
		// For development/debugging - traces will be printed to stdout
		exporter, err = newStdoutExporter()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", exporterType)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set global propagator to tracecontext (W3C Trace Context)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Return shutdown function
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			setupLog.Error(err, "failed to shutdown trace provider")
		}
	}, nil
}

// Simple stdout exporter for development
func newStdoutExporter() (sdktrace.SpanExporter, error) {
	return stdouttrace.New(stdouttrace.WithPrettyPrint())
}
