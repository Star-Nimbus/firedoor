package telemetry

import (
	"context"
	"fmt"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// SetupOTel initializes OpenTelemetry tracing and returns a tracer provider
func SetupOTel(ctx context.Context, exporterType, endpoint, serviceName string) (*sdktrace.TracerProvider, error) {
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
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithInsecure(),
		}
		if endpoint != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(endpoint))
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP gRPC exporter: %w", err)
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

	// Set global propagator to tracecontext (W3C Trace Context)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp, nil
}

// Simple stdout exporter for development
func newStdoutExporter() (sdktrace.SpanExporter, error) {
	return stdouttrace.New(stdouttrace.WithPrettyPrint())
}

// WithSpan is a helper function to create a span and end it when the function returns
func WithSpan(ctx context.Context, tracer trace.Tracer, name string, f func(ctx context.Context) error) error {
	ctx, span := tracer.Start(ctx, name)
	defer span.End()

	return f(ctx)
}
