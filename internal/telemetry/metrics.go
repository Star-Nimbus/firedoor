/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package telemetry

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/cloud-nimbus/firedoor/internal/config"
)

// Metric name constants
const (
	// Breakglass metric names
	MetricBreakglassTotal             = "firedoor_breakglass_total"
	MetricBreakglassActive            = "firedoor_breakglass_active"
	MetricBreakglassExpired           = "firedoor_breakglass_expired"
	MetricBreakglassDuration          = "firedoor_breakglass_duration_minutes"
	MetricBreakglassReconcileTotal    = "firedoor_breakglass_reconcile_total"
	MetricBreakglassReconcileDuration = "firedoor_breakglass_reconcile_duration_seconds"
	MetricBreakglassCreationTotal     = "firedoor_breakglass_creation_total"
	MetricBreakglassDeletionTotal     = "firedoor_breakglass_deletion_total"
	MetricBreakglassApprovalTotal     = "firedoor_breakglass_approval_total"
	MetricBreakglassRoleBindingTotal  = "firedoor_breakglass_role_binding_total"
	MetricBreakglassValidationTotal   = "firedoor_breakglass_validation_total"
	MetricBreakglassErrorTotal        = "firedoor_breakglass_error_total"
)

// Help string constants
const (
	// Breakglass lifecycle help strings
	HelpBreakglassTotal    = "Total number of breakglass requests by phase and reason"
	HelpBreakglassActive   = "Current number of active breakglass requests"
	HelpBreakglassExpired  = "Total number of expired breakglass requests"
	HelpBreakglassDuration = "Duration in minutes for breakglass requests"

	// Breakglass operation help strings
	HelpBreakglassCreationTotal    = "Total number of breakglass creation operations"
	HelpBreakglassDeletionTotal    = "Total number of breakglass deletion operations"
	HelpBreakglassApprovalTotal    = "Total number of breakglass approval operations"
	HelpBreakglassRoleBindingTotal = "Total number of role binding operations for breakglass requests"
	HelpBreakglassValidationTotal  = "Total number of breakglass validation operations"
	HelpBreakglassErrorTotal       = "Total number of errors encountered during breakglass operations"

	// Reconciliation help strings
	HelpBreakglassReconcileTotal    = "Total number of breakglass reconciliation operations"
	HelpBreakglassReconcileDuration = "Duration of breakglass reconciliation operations in seconds"
)

// Label constants for breakglass metrics
const (
	// Phase labels
	LabelPhasePending = "pending"
	LabelPhaseActive  = "active"
	LabelPhaseExpired = "expired"
	LabelPhaseDenied  = "denied"
	LabelPhaseRevoked = "revoked"

	// Result labels
	LabelResultSuccess = "success"
	LabelResultError   = "error"
	LabelResultFailure = "failure"

	// Reason labels
	LabelReasonGranted     = "granted"
	LabelReasonDenied      = "denied"
	LabelReasonExpired     = "expired"
	LabelReasonRevoked     = "revoked"
	LabelReasonManual      = "manual"
	LabelReasonAutomatic   = "automatic"
	LabelReasonValidation  = "validation_failed"
	LabelReasonApproval    = "approval_required"
	LabelReasonRoleBinding = "role_binding_failed"
	LabelReasonTimeout     = "timeout"
	LabelReasonConflict    = "conflict"

	// Operation labels
	LabelOperationCreate    = "create"
	LabelOperationUpdate    = "update"
	LabelOperationDelete    = "delete"
	LabelOperationReconcile = "reconcile"
	LabelOperationApprove   = "approve"
	LabelOperationDeny      = "deny"
	LabelOperationRevoke    = "revoke"

	// Component labels
	LabelComponentController = "controller"
	LabelComponentWebhook    = "webhook"
	LabelComponentAPI        = "api"
	LabelComponentMetrics    = "metrics"

	// Error type labels
	LabelErrorTypeValidation    = "validation"
	LabelErrorTypeAuthorization = "authorization"
	LabelErrorTypeInternal      = "internal"
	LabelErrorTypeNetwork       = "network"
	LabelErrorTypeTimeout       = "timeout"
	LabelErrorTypeConflict      = "conflict"
)

// Metric label names
const (
	LabelNamePhase      = "phase"
	LabelNameResult     = "result"
	LabelNameReason     = "reason"
	LabelNameOperation  = "operation"
	LabelNameComponent  = "component"
	LabelNameErrorType  = "error_type"
	LabelNameNamespace  = "namespace"
	LabelNameUser       = "user"
	LabelNameGroup      = "group"
	LabelNameRole       = "role"
	LabelNameApprovedBy = "approved_by"
)

var (
	// Metrics initialization state
	metricsInitialized bool
	metricsLock        sync.Mutex

	// Breakglass lifecycle metrics
	BreakglassTotal    *prometheus.CounterVec
	BreakglassActive   prometheus.Gauge
	BreakglassExpired  prometheus.Counter
	BreakglassDuration prometheus.Histogram

	// Breakglass operation metrics
	BreakglassCreationTotal    *prometheus.CounterVec
	BreakglassDeletionTotal    *prometheus.CounterVec
	BreakglassApprovalTotal    *prometheus.CounterVec
	BreakglassRoleBindingTotal *prometheus.CounterVec
	BreakglassValidationTotal  *prometheus.CounterVec
	BreakglassErrorTotal       *prometheus.CounterVec

	// Reconciliation metrics
	BreakglassReconcileTotal    *prometheus.CounterVec
	BreakglassReconcileDuration prometheus.Histogram
)

// InitializeMetrics initializes all metrics with the given configuration
func InitializeMetrics(cfg *config.Config) {
	metricsLock.Lock()
	defer metricsLock.Unlock()

	if metricsInitialized {
		return
	}

	// Breakglass lifecycle metrics
	BreakglassTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricBreakglassTotal,
			Help: HelpBreakglassTotal,
		},
		[]string{LabelNamePhase, LabelNameReason, LabelNameNamespace, LabelNameRole},
	)

	BreakglassActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricBreakglassActive,
			Help: HelpBreakglassActive,
		},
	)

	BreakglassExpired = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: MetricBreakglassExpired,
			Help: HelpBreakglassExpired,
		},
	)

	BreakglassDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    MetricBreakglassDuration,
			Help:    HelpBreakglassDuration,
			Buckets: cfg.GetDurationBuckets(),
		},
	)

	// Breakglass operation metrics
	BreakglassCreationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricBreakglassCreationTotal,
			Help: HelpBreakglassCreationTotal,
		},
		[]string{LabelNameResult, LabelNameComponent, LabelNameNamespace},
	)

	BreakglassDeletionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricBreakglassDeletionTotal,
			Help: HelpBreakglassDeletionTotal,
		},
		[]string{LabelNameResult, LabelNameComponent, LabelNameNamespace},
	)

	BreakglassApprovalTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricBreakglassApprovalTotal,
			Help: HelpBreakglassApprovalTotal,
		},
		[]string{LabelNameResult, LabelNameOperation, LabelNameApprovedBy},
	)

	BreakglassRoleBindingTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricBreakglassRoleBindingTotal,
			Help: HelpBreakglassRoleBindingTotal,
		},
		[]string{LabelNameResult, LabelNameOperation, LabelNameRole, LabelNameNamespace},
	)

	BreakglassValidationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricBreakglassValidationTotal,
			Help: HelpBreakglassValidationTotal,
		},
		[]string{LabelNameResult, LabelNameComponent, LabelNameErrorType},
	)

	BreakglassErrorTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricBreakglassErrorTotal,
			Help: HelpBreakglassErrorTotal,
		},
		[]string{LabelNameComponent, LabelNameErrorType, LabelNameOperation},
	)

	// Reconciliation metrics
	BreakglassReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricBreakglassReconcileTotal,
			Help: HelpBreakglassReconcileTotal,
		},
		[]string{LabelNameResult, LabelNamePhase, LabelNameNamespace},
	)

	BreakglassReconcileDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    MetricBreakglassReconcileDuration,
			Help:    HelpBreakglassReconcileDuration,
			Buckets: prometheus.DefBuckets,
		},
	)

	// Register all metrics with the controller-runtime metrics registry
	metrics.Registry.MustRegister(
		BreakglassTotal,
		BreakglassActive,
		BreakglassExpired,
		BreakglassDuration,
		BreakglassCreationTotal,
		BreakglassDeletionTotal,
		BreakglassApprovalTotal,
		BreakglassRoleBindingTotal,
		BreakglassValidationTotal,
		BreakglassErrorTotal,
		BreakglassReconcileTotal,
		BreakglassReconcileDuration,
	)

	metricsInitialized = true
}

// init provides default initialization for backward compatibility
func init() {
	// Initialize with default config if not already initialized
	defaultConfig := config.NewDefaultConfig()
	InitializeMetrics(defaultConfig)
}

// RecordBreakglassGranted records metrics when breakglass access is granted
func RecordBreakglassGranted(namespace, role string, durationMinutes int, approvedBy string) {
	if BreakglassTotal != nil {
		BreakglassTotal.WithLabelValues(LabelPhaseActive, LabelReasonGranted, namespace, role).Inc()
	}
	if BreakglassActive != nil {
		BreakglassActive.Inc()
	}
	if BreakglassDuration != nil {
		BreakglassDuration.Observe(float64(durationMinutes))
	}
	if BreakglassApprovalTotal != nil {
		BreakglassApprovalTotal.WithLabelValues(LabelResultSuccess, LabelOperationApprove, approvedBy).Inc()
	}
}

// RecordBreakglassDenied records metrics when breakglass access is denied
func RecordBreakglassDenied(namespace, role, reason, deniedBy string) {
	if BreakglassTotal != nil {
		BreakglassTotal.WithLabelValues(LabelPhaseDenied, reason, namespace, role).Inc()
	}
	if BreakglassApprovalTotal != nil {
		BreakglassApprovalTotal.WithLabelValues(LabelResultSuccess, LabelOperationDeny, deniedBy).Inc()
	}
}

// RecordBreakglassExpired records metrics when breakglass access expires
func RecordBreakglassExpired(namespace, role string) {
	if BreakglassExpired != nil {
		BreakglassExpired.Inc()
	}
	if BreakglassActive != nil {
		BreakglassActive.Dec()
	}
	if BreakglassTotal != nil {
		BreakglassTotal.WithLabelValues(LabelPhaseExpired, LabelReasonExpired, namespace, role).Inc()
	}
}

// RecordBreakglassRevoked records metrics when breakglass access is manually revoked
func RecordBreakglassRevoked(namespace, role, revokedBy string) {
	if BreakglassTotal != nil {
		BreakglassTotal.WithLabelValues(LabelPhaseRevoked, LabelReasonRevoked, namespace, role).Inc()
	}
	if BreakglassActive != nil {
		BreakglassActive.Dec()
	}
	if BreakglassApprovalTotal != nil {
		BreakglassApprovalTotal.WithLabelValues(LabelResultSuccess, LabelOperationRevoke, revokedBy).Inc()
	}
}

// RecordBreakglassCreation records metrics for breakglass creation operations
func RecordBreakglassCreation(result, component, namespace string) {
	if BreakglassCreationTotal != nil {
		BreakglassCreationTotal.WithLabelValues(result, component, namespace).Inc()
	}
}

// RecordBreakglassDeletion records metrics for breakglass deletion operations
func RecordBreakglassDeletion(result, component, namespace string) {
	if BreakglassDeletionTotal != nil {
		BreakglassDeletionTotal.WithLabelValues(result, component, namespace).Inc()
	}
}

// RecordRoleBindingOperation records metrics for role binding operations
func RecordRoleBindingOperation(result, operation, role, namespace string) {
	if BreakglassRoleBindingTotal != nil {
		BreakglassRoleBindingTotal.WithLabelValues(result, operation, role, namespace).Inc()
	}
}

// RecordValidationOperation records metrics for validation operations
func RecordValidationOperation(result, component, errorType string) {
	if BreakglassValidationTotal != nil {
		BreakglassValidationTotal.WithLabelValues(result, component, errorType).Inc()
	}
}

// RecordError records metrics for errors encountered during operations
func RecordError(component, errorType, operation string) {
	if BreakglassErrorTotal != nil {
		BreakglassErrorTotal.WithLabelValues(component, errorType, operation).Inc()
	}
}

// RecordReconcileResult records the result of a reconciliation operation
func RecordReconcileResult(result, phase, namespace string) {
	if BreakglassReconcileTotal != nil {
		BreakglassReconcileTotal.WithLabelValues(result, phase, namespace).Inc()
	}
}

// GetReconcileDurationTimer returns a timer for measuring reconciliation duration
func GetReconcileDurationTimer() *prometheus.Timer {
	if BreakglassReconcileDuration != nil {
		return prometheus.NewTimer(BreakglassReconcileDuration)
	}
	// Return a no-op timer if metrics aren't initialized
	return &prometheus.Timer{}
}
