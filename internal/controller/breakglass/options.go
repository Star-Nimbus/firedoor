package breakglass

import (
	"k8s.io/client-go/tools/record"

	"github.com/cloud-nimbus/firedoor/internal/controller"
)

// Option configures the BreakglassReconciler.
type Option func(*BreakglassReconciler)

// WithOperator injects your concrete RBAC operator.
func WithOperator(operator controller.BreakglassOperator) Option {
	return func(r *BreakglassReconciler) {
		r.Operator = operator
	}
}

// WithClock injects a Clock implementation.
func WithClock(clock controller.Clock) Option {
	return func(r *BreakglassReconciler) {
		r.Clock = clock
	}
}

// WithAlerts injects an AlertService implementation.
func WithAlerts(alerts controller.AlertService) Option {
	return func(r *BreakglassReconciler) {
		r.Alerts = alerts
	}
}

// WithRecurringManager injects a RecurringManager implementation.
func WithRecurringManager(manager controller.RecurringManager) Option {
	return func(r *BreakglassReconciler) {
		r.RecurringManager = manager
	}
}

// WithTelemetry injects a TelemetrySink implementation.
func WithTelemetry(telemetry controller.TelemetrySink) Option {
	return func(r *BreakglassReconciler) {
		r.Telemetry = telemetry
	}
}

// WithEventRecorder injects an EventRecorder implementation.
func WithEventRecorder(recorder record.EventRecorder) Option {
	return func(r *BreakglassReconciler) {
		r.recorder = recorder
	}
}
