package metrics

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
)

const roleTypeBreakglass = "breakglass"

var tracer = otel.Tracer("firedoor/internal/telemetry/breakglass")

// ReconciliationResult represents the result of a reconciliation operation
type ReconciliationResult string

const (
	// ReconciliationResultNotFound indicates the resource was not found
	ReconciliationResultNotFound ReconciliationResult = "not_found"
	// ReconciliationResultNoAction indicates no action was taken
	ReconciliationResultNoAction ReconciliationResult = "no_action"
	// ReconciliationResultError indicates an error occurred
	ReconciliationResultError ReconciliationResult = "error"
)

// ReconciliationPhase represents the phase of a reconciliation operation
type ReconciliationPhase string

const (
	// ReconciliationPhaseExpired indicates the breakglass has expired
	ReconciliationPhaseExpired ReconciliationPhase = "expired"
	// ReconciliationPhaseActive indicates the breakglass is active
	ReconciliationPhaseActive ReconciliationPhase = "active"
)

// AttributeKey represents OpenTelemetry attribute keys
type AttributeKey string

const (
	// AttributeKeyBreakglassName is the key for breakglass name attribute
	AttributeKeyBreakglassName AttributeKey = "breakglass.name"
	// AttributeKeyBreakglassUID is the key for breakglass UID attribute
	AttributeKeyBreakglassUID AttributeKey = "breakglass.uid"
	// AttributeKeyBreakglassNamespaces is the key for breakglass namespaces attribute (bounded array)
	AttributeKeyBreakglassNamespaces AttributeKey = "breakglass.namespaces"
	// AttributeKeyBreakglassRoleType is the key for breakglass role type attribute
	AttributeKeyBreakglassRoleType AttributeKey = "breakglass.role_type"
	// AttributeKeyBreakglassDurationMinutes is the key for breakglass duration minutes attribute
	AttributeKeyBreakglassDurationMinutes AttributeKey = "breakglass.duration_minutes"
	// AttributeKeyBreakglassApprovalRequired is the key for breakglass approval required attribute
	AttributeKeyBreakglassApprovalRequired AttributeKey = "breakglass.approval_required"
	// AttributeKeyBreakglassRecurring is the key for breakglass recurring attribute
	AttributeKeyBreakglassRecurring AttributeKey = "breakglass.recurring"
	// AttributeKeySubject is the key for subject attribute
	AttributeKeySubject AttributeKey = "subject"
	// AttributeKeyNamespace is the key for namespace attribute (used in child spans)
	AttributeKeyNamespace AttributeKey = "namespace"
)

// String returns the string representation of the attribute key
func (ak AttributeKey) String() string {
	return string(ak)
}

// Operation names for OpenTelemetry spans
const (
	// OperationNameGrantAccess is the name for grant access operations
	OperationNameGrantAccess = "grantAccess"
	// OperationNameRevokeAccess is the name for revoke access operations
	OperationNameRevokeAccess = "revokeAccess"
	// OperationNameRecurringBreakglassActivation is the name for recurring breakglass activation operations
	OperationNameRecurringBreakglassActivation = "recurringBreakglassActivation"
	// OperationNameRecurringBreakglassExpiration is the name for recurring breakglass expiration operations
	OperationNameRecurringBreakglassExpiration = "recurringBreakglassExpiration"
)

// AutoApprover is the constant for system auto-approval
const AutoApprover = "system-auto-approve"

// Helper functions for bounded metrics - NO HIGH CARDINALITY
func getApprovalSource(approvedBy string) string {
	if approvedBy == "" {
		return "auto"
	}
	if approvedBy == AutoApprover {
		return "auto"
	}
	return "human"
}

// Helper functions to extract values from the new API structure
func namespaceKey(bg *accessv1alpha1.Breakglass) string {
	all := getAllNamespaces(bg)
	if len(all) == 1 {
		return all[0]
	}
	return "multi"
}

func getDurationMinutes(bg *accessv1alpha1.Breakglass) int {
	return int(bg.Spec.Schedule.Duration.Minutes())
}

func setBreakglassSpanAttributes(span trace.Span, bg *accessv1alpha1.Breakglass) {
	namespaces := getAllNamespaces(bg)
	nsAttr := attribute.StringSlice("breakglass.namespaces", namespaces)

	span.SetAttributes(
		attribute.String(AttributeKeyBreakglassName.String(), bg.Name),
		attribute.String(AttributeKeyBreakglassUID.String(), string(bg.UID)),
		nsAttr, // All namespaces as bounded array
		attribute.Int(AttributeKeyBreakglassDurationMinutes.String(), getDurationMinutes(bg)),
		attribute.Bool(
			AttributeKeyBreakglassApprovalRequired.String(),
			bg.Spec.Approval != nil && bg.Spec.Approval.Required,
		),
		attribute.Bool(AttributeKeyBreakglassRecurring.String(), bg.Spec.Schedule.Cron != ""),
	)
}

// RecordGrantAccessStart records the start of a grant access operation
// Uses bounded string array for namespaces to preserve all namespace information
func RecordGrantAccessStart(ctx context.Context, bg *accessv1alpha1.Breakglass) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, OperationNameGrantAccess)
	setBreakglassSpanAttributes(span, bg)
	return ctx, span
}

// RecordRevokeAccessStart records the start of a revoke access operation
// Uses bounded string array for namespaces to preserve all namespace information
func RecordRevokeAccessStart(ctx context.Context, bg *accessv1alpha1.Breakglass) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, OperationNameRevokeAccess)
	setBreakglassSpanAttributes(span, bg)
	return ctx, span
}

// RecordSubjectResolution records subject resolution for a breakglass
func RecordSubjectResolution(ctx context.Context, bg *accessv1alpha1.Breakglass, subjectName string) {
	_, span := tracer.Start(ctx, "subjectResolution")
	defer span.End()

	span.SetAttributes(
		attribute.String(AttributeKeyBreakglassName.String(), bg.Name),
		attribute.String(AttributeKeySubject.String(), subjectName),
	)
}

// CreateNamespaceChildSpan creates a child span for namespace-specific operations
func CreateNamespaceChildSpan(ctx context.Context, operationName, namespace string) (context.Context, trace.Span) {
	return tracer.Start(ctx, operationName, trace.WithAttributes(
		attribute.String(AttributeKeyNamespace.String(), namespace),
	))
}

// RecordNamespaceOperation records a namespace-specific operation with telemetry
func RecordNamespaceOperation(ctx context.Context, operationName, namespace string, operation func() error) error {
	_, span := CreateNamespaceChildSpan(ctx, operationName, namespace)
	defer span.End()

	err := operation()
	if err != nil {
		span.RecordError(err)
	}
	return err
}

// RecordGrantAccessSuccess records successful grant access with new collapsed metrics
func RecordGrantAccessSuccess(bg *accessv1alpha1.Breakglass, subjectName string) {
	// Record state transition to active
	approvalSource := getApprovalSource("system") // TODO: Extract from conditions if needed
	roleType := roleTypeBreakglass
	namespace := namespaceKey(bg)

	RecordStateTransition("active", approvalSource, roleType, 1) // +1 to active gauge

	// Record operation success
	RecordOperation(OpCreate, ResultSuccess, ComponentController, roleType, namespace)
}

// RecordGrantAccessFailure records failed grant access with new collapsed metrics
func RecordGrantAccessFailure(bg *accessv1alpha1.Breakglass, reason string) {
	// Record operation failure
	roleType := roleTypeBreakglass
	namespace := namespaceKey(bg)

	RecordOperation(OpCreate, ResultError, ComponentController, roleType, namespace)
}

// RecordGrantAccessValidationFailure records validation failure with new collapsed metrics
func RecordGrantAccessValidationFailure(bg *accessv1alpha1.Breakglass) {
	// Record validation operation failure
	roleType := roleTypeBreakglass
	namespace := namespaceKey(bg)

	RecordOperation(OpValidation, ResultError, ComponentController, roleType, namespace)
}

// RecordRevokeAccessSuccess records successful revoke access with new collapsed metrics
func RecordRevokeAccessSuccess(bg *accessv1alpha1.Breakglass) {
	// Record state transition (decrease active gauge)
	approvalSource := getApprovalSource("system") // TODO: Extract from conditions if needed
	roleType := roleTypeBreakglass
	namespace := namespaceKey(bg)

	RecordStateTransition("revoked", approvalSource, roleType, -1) // -1 from active gauge

	// Record operation success
	RecordOperation(OpRevoke, ResultSuccess, ComponentController, roleType, namespace)
}

// RecordRevokeAccessFailure records failed revoke access with new collapsed metrics
func RecordRevokeAccessFailure(bg *accessv1alpha1.Breakglass) {
	// Record operation failure
	roleType := roleTypeBreakglass
	namespace := namespaceKey(bg)

	RecordOperation(OpRevoke, ResultError, ComponentController, roleType, namespace)
}

// RecordStatusUpdateError records status update errors with new collapsed metrics
func RecordStatusUpdateError(operation string) {
	// Record operation error
	RecordOperation(Op(operation), ResultError, ComponentController, "unknown", "")
}

// RecordReconciliationNotFound records reconciliation not found with new collapsed metrics
func RecordReconciliationNotFound(namespace string) {
	// Record operation success (not found is expected)
	RecordOperation(OpReconcile, ResultSuccess, ComponentController, "unknown", namespace)
}

// RecordReconciliationError records reconciliation errors with new collapsed metrics
func RecordReconciliationError(namespace string) {
	// Record operation error
	RecordOperation(OpReconcile, ResultError, ComponentController, "unknown", namespace)
}

// RecordReconciliationNoAction records reconciliation no action with new collapsed metrics
func RecordReconciliationNoAction(namespace string) {
	// Record operation success (no action is expected)
	RecordOperation(OpReconcile, ResultSuccess, ComponentController, "unknown", namespace)
}

// RecordReconciliationExpired records reconciliation expired with new collapsed metrics
func RecordReconciliationExpired(namespace string, success bool) {
	result := ResultSuccess
	if !success {
		result = ResultError
	}

	// Record operation
	RecordOperation(OpDelete, result, ComponentController, "unknown", namespace)
}

// RecordReconciliationActive records reconciliation active with new collapsed metrics
func RecordReconciliationActive(namespace string, success bool) {
	result := ResultSuccess
	if !success {
		result = ResultError
	}

	// Record operation
	RecordOperation(OpCreate, result, ComponentController, "unknown", namespace)
}

// RecordRecurringBreakglassActivationWithTelemetry records recurring breakglass activation with new collapsed metrics
func RecordRecurringBreakglassActivationWithTelemetry(namespace string, activationCount int32) {
	// Record recurring activation
	RecordRecurringActivation(namespace)

	// Record operation success
	RecordOperation(OpReconcile, ResultSuccess, ComponentController, "unknown", namespace)
}

// RecordRecurringBreakglassExpirationWithTelemetry records recurring breakglass expiration with new collapsed metrics
func RecordRecurringBreakglassExpirationWithTelemetry(namespace string, activationCount int32) {
	// Record recurring expiration
	RecordRecurringExpiration(namespace)

	// Record operation success
	RecordOperation(OpDelete, ResultSuccess, ComponentController, "unknown", namespace)
}
