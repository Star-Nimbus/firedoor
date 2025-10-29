package metrics

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// SetupPrometheus registers an OTEL Prometheus exporter into the given registry,
// and installs it as the global MeterProvider.
func SetupPrometheus(reg ctrlmetrics.RegistererGatherer) error {
	// 1) Create the exporter bound to controller-runtime's registry
	exp, err := prometheus.New(prometheus.WithRegisterer(reg))
	if err != nil {
		return fmt.Errorf("prometheus exporter: %w", err)
	}

	// 2) Hook it into the MeterProvider
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exp),
	)
	otel.SetMeterProvider(provider)

	return nil
}
