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
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/config"
	"go.opentelemetry.io/otel/trace"
)

// Metric names
const (
	MetricBreakglassStateTotal        = "firedoor_breakglass_state_total"                // counter
	MetricBreakglassActive            = "firedoor_breakglass_active"                     // gauge
	MetricBreakglassDurationSeconds   = "firedoor_breakglass_duration_seconds"           // histogram
	MetricBreakglassOperationsTotal   = "firedoor_breakglass_operations_total"           // counter vec
	MetricBreakglassReconcileDuration = "firedoor_breakglass_reconcile_duration_seconds" // histogram

	// Recurring
	MetricRecurringActivationTotal = "firedoor_recurring_breakglass_activation_total"
	MetricRecurringExpirationTotal = "firedoor_recurring_breakglass_expiration_total"
	MetricRecurringActive          = "firedoor_recurring_breakglass_active"

	// Alerting
	MetricAlertsSentTotal   = "firedoor_alerts_sent_total"
	MetricAlertSendDuration = "firedoor_alert_send_duration_seconds"
	MetricAlertSendErrors   = "firedoor_alert_send_errors_total"
)

// Label names – ALL BOUNDED ENUMS
const (
	// state_total
	LPhase          = "phase"           // pending|active|expired|denied|failed|revoked|recurring_pending|recurring_active
	LApprovalSource = "approval_source" // human|auto
	LRoleType       = "role_type"       // cluster_role|custom|unknown

	// operations_total
	LOperation       = "operation"        // create|delete|approve|deny|revoke|reconcile|rolebinding|validation
	LResult          = "result"           // success|error
	LComponent       = "component"        // controller|webhook|api
	LNamespaceBucket = "namespace_bucket" // ns_00‑ns_0f
	LApproverBucket  = "approver_bucket"  // ap_00‑ap_0f

	// alerting
	LAlertType = "alert_type" // active|expired
	LSeverity  = "severity"   // warning|critical|info

	// reconcile_duration seconds histogram needs no extra labels

	RoleUnknown = "unknown"
)

// -----------------------------------------------------------------------------
//
//	Public, ready‑to‑use Prometheus collectors
//
// -----------------------------------------------------------------------------
var (
	stateTotal        *prometheus.CounterVec
	activeGauge       prometheus.Gauge
	durationHist      prometheus.Histogram
	operationsTotal   *prometheus.CounterVec
	reconcileDuration prometheus.Histogram

	// recurring
	recurringActivationTotal *prometheus.CounterVec
	recurringExpirationTotal *prometheus.CounterVec
	recurringActiveGauge     prometheus.Gauge

	// alerting
	alertsSentTotal   *prometheus.CounterVec
	alertSendDuration *prometheus.HistogramVec
	alertSendErrors   *prometheus.CounterVec

	initOnce sync.Once
)

// Init registers all collectors exactly once.
// Call from main() *or* let the implicit init() below run.
func Init(cfg *config.Config) {
	initOnce.Do(func() { register(cfg) })
}

// -----------------------------------------------------------------------------
//
//	Collector construction & registration
//
// -----------------------------------------------------------------------------
func register(cfg *config.Config) {
	// --- lifecycle -----------------------------------------------------------
	stateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: MetricBreakglassStateTotal, Help: "Number of breakglass requests by phase"},
		[]string{LPhase, LApprovalSource, LRoleType},
	)

	activeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricBreakglassActive, Help: "Current active breakglass sessions"})

	durationHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    MetricBreakglassDurationSeconds,
		Help:    "Observed breakglass duration seconds",
		Buckets: cfg.GetDurationBuckets(),
	})

	// --- operations ----------------------------------------------------------
	operationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: MetricBreakglassOperationsTotal, Help: "CRUD & workflow operations"},
		[]string{LOperation, LResult, LComponent, LRoleType, LNamespaceBucket},
	)

	// --- reconcile latency ---------------------------------------------------
	reconcileDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    MetricBreakglassReconcileDuration,
		Help:    "Reconcile duration seconds",
		Buckets: prometheus.DefBuckets,
	})

	// --- recurring -----------------------------------------------------------
	recurringActivationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: MetricRecurringActivationTotal, Help: "Recurring breakglass activations"},
		[]string{LNamespaceBucket},
	)
	recurringExpirationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: MetricRecurringExpirationTotal, Help: "Recurring breakglass expirations"},
		[]string{LNamespaceBucket},
	)
	recurringActiveGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricRecurringActive, Help: "Current active recurring breakglass sessions"})

	// --- alerting ------------------------------------------------------------
	alertsSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: MetricAlertsSentTotal, Help: "Total number of alerts sent to Alertmanager"},
		[]string{LAlertType, LSeverity, LNamespaceBucket},
	)
	alertSendDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    MetricAlertSendDuration,
			Help:    "Duration of alert send operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{LAlertType, LSeverity},
	)
	alertSendErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: MetricAlertSendErrors, Help: "Total number of alert send errors"},
		[]string{LAlertType, LSeverity, LNamespaceBucket},
	)

	collectors := []prometheus.Collector{
		stateTotal, activeGauge, durationHist,
		operationsTotal, reconcileDuration,
		recurringActivationTotal, recurringExpirationTotal, recurringActiveGauge,
		alertsSentTotal, alertSendDuration, alertSendErrors,
	}
	metrics.Registry.MustRegister(collectors...)
}

// -----------------------------------------------------------------------------
//  Recording helpers (public API)
// -----------------------------------------------------------------------------

// RecordStateTransition increments phase counters and adjusts the active gauge.
func RecordStateTransition(phase, approvalSrc, roleType string, activeDelta int) {
	stateTotal.WithLabelValues(phase, approvalSrc, roleType).Inc()
	if activeDelta != 0 {
		activeGauge.Add(float64(activeDelta))
	}
}

// ObserveDurationSeconds records an observed session duration.
func ObserveDurationSeconds(sec float64) { durationHist.Observe(sec) }

// ObserveReconcileDurationSeconds records reconciliation latency.
func ObserveReconcileDurationSeconds(sec float64) { reconcileDuration.Observe(sec) }

// RecordOperation emits a single operation counter.
// operation: create|delete|approve|deny|revoke|reconcile|rolebinding|validation
// result:    success|error
func RecordOperation(op Op, result Result, component Component, roleType, namespace string) {
	nb := namespaceBucket(namespace)
	operationsTotal.WithLabelValues(string(op), string(result), string(component), roleType, nb).Inc()
}

// Recurring helpers -----------------------------------------------------------------
func RecordRecurringActivation(namespace string) {
	nb := namespaceBucket(namespace)
	recurringActivationTotal.WithLabelValues(nb).Inc()
	recurringActiveGauge.Inc()
}

func RecordRecurringExpiration(namespace string) {
	nb := namespaceBucket(namespace)
	recurringExpirationTotal.WithLabelValues(nb).Inc()
	recurringActiveGauge.Dec()
}

// Alerting helpers -----------------------------------------------------------------
func RecordAlertSent(alertType, severity, namespace string, duration float64) {
	nb := namespaceBucket(namespace)
	alertsSentTotal.WithLabelValues(alertType, severity, nb).Inc()
	alertSendDuration.WithLabelValues(alertType, severity).Observe(duration)
}

func RecordAlertSendError(alertType, severity, namespace string) {
	nb := namespaceBucket(namespace)
	alertSendErrors.WithLabelValues(alertType, severity, nb).Inc()
}

// -----------------------------------------------------------------------------
//  Bucketing helpers  (keep cardinality ≤ 16)
// -----------------------------------------------------------------------------

// namespaceBucket returns a bucketed namespace label for metrics.
func namespaceBucket(ns string) string {
	if ns == "" {
		ns = "default"
	}
	h := fnv.New32a()
	h.Write([]byte(ns))
	return fmt.Sprintf("ns_%02x", h.Sum32()&0x0f)
}

// NamespaceBucket exposes the namespace bucketing algorithm.
// It returns a label like "ns_0a" for the provided namespace.
func NamespaceBucket(ns string) string { return namespaceBucket(ns) }

// ApproverBucket hashes an approver ID/email into 16 buckets (ap_00..ap_0f).
func ApproverBucket(approver string) string {
	if approver == "" {
		return "ap_00"
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(approver))
	return fmt.Sprintf("ap_%02x", h.Sum32()&0x0f)
}

// -----------------------------------------------------------------------------
//
//	Package‑level init for zero‑config usage -----------------------------------
//
// -----------------------------------------------------------------------------
func init() { Init(config.NewDefaultConfig()) }

// NamespaceKey returns a single representative key for metric labelling.
// Exported version of namespaceKey for use in controllers.
func NamespaceKey(bg *accessv1alpha1.Breakglass) string {
	all := getAllNamespaces(bg)
	if len(all) == 1 {
		return all[0]
	}
	return "multi"
}

// ObserveReconcileDurationSecondsWithExemplar records reconciliation latency with trace exemplar
func ObserveReconcileDurationSecondsWithExemplar(sec float64, span trace.Span) {
	observeHistogramWithExemplar(reconcileDuration, sec, span)
}

// observeHistogramWithExemplar tries to attach a trace exemplar, but
// degrades gracefully if:
//   - the histogram doesn't implement ExemplarObserver (old lib), or
//   - the span is not sampled.
func observeHistogramWithExemplar(
	h prometheus.Histogram,
	val float64,
	span trace.Span,
) {
	// Always record the raw observation
	h.Observe(val)

	// Only attach exemplars for sampled traces
	if span == nil || !span.SpanContext().IsSampled() {
		return
	}

	if eo, ok := h.(prometheus.ExemplarObserver); ok {
		eo.ObserveWithExemplar(val, prometheus.Labels{
			"trace_id": span.SpanContext().TraceID().String(),
			"span_id":  span.SpanContext().SpanID().String(),
		})
	}
}
