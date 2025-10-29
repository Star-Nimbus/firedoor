package controller

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
)

// BreakglassOperator handles RBAC operations for breakglass requests
type BreakglassOperator interface {
	GrantAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) error
	RevokeAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) error
	ValidateAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) error
	CleanupResources(ctx context.Context, bg *accessv1alpha1.Breakglass) error
}

// RecurringManager handles recurring breakglass schedules
type RecurringManager interface {
	ProcessRecurring(ctx context.Context, bg *accessv1alpha1.Breakglass) error
	ShouldActivate(ctx context.Context, bg *accessv1alpha1.Breakglass) bool
	ShouldDeactivate(ctx context.Context, bg *accessv1alpha1.Breakglass) bool
	OnActivationGranted(ctx context.Context, bg *accessv1alpha1.Breakglass) error
}

// AlertService handles alerting operations
type AlertService interface {
	SendAlert(ctx context.Context, bg *accessv1alpha1.Breakglass, alertType string) error
}

// Clock provides time-related operations
type Clock interface {
	Now() time.Time
	Until(t time.Time) time.Duration
	IsExpired(t time.Time) bool
}

// TelemetrySink handles telemetry operations
type TelemetrySink interface {
	RecordEvent(ctx context.Context, bg *accessv1alpha1.Breakglass, eventType string) error
	RecordMetrics(ctx context.Context, bg *accessv1alpha1.Breakglass, metrics map[string]float64) error
}

// Handler handles breakglass phase transitions
type Handler interface {
	Handle(ctx context.Context, bg *accessv1alpha1.Breakglass) (reconcile.Result, error)
}
