package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// SetupTracing initializes OpenTelemetry tracing with the given configuration
func SetupTracing(
	ctx context.Context,
	exporterType, endpoint, serviceName, serviceVersion string,
	insecure bool,
) (*sdktrace.TracerProvider, error) {
	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize exporter based on type
	var exporter sdktrace.SpanExporter
	switch exporterType {
	case "otlp":
		opts := []otlptracegrpc.Option{}
		if insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
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
		exporter, err = stdouttrace.New()
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
