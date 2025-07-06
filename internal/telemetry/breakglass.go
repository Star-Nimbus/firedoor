package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/constants"
)

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

// Attribute keys for OpenTelemetry spans
const (
	// AttributeKeyRequestNamespace is the key for request namespace attribute
	AttributeKeyRequestNamespace = "request.namespace"
	// AttributeKeyRequestName is the key for request name attribute
	AttributeKeyRequestName = "request.name"
	// AttributeKeyBreakglassName is the key for breakglass name attribute
	AttributeKeyBreakglassName = "breakglass.name"
	// AttributeKeyBreakglassRole is the key for breakglass role attribute
	AttributeKeyBreakglassRole = "breakglass.role"
	// AttributeKeyBreakglassDurationMinutes is the key for breakglass duration minutes attribute
	AttributeKeyBreakglassDurationMinutes = "breakglass.duration_minutes"
	// AttributeKeySubject is the key for subject attribute
	AttributeKeySubject = "subject"
)

// Operation names for OpenTelemetry spans
const (
	// OperationNameGrantAccess is the name for grant access operations
	OperationNameGrantAccess = "grantAccess"
	// OperationNameRevokeAccess is the name for revoke access operations
	OperationNameRevokeAccess = "revokeAccess"
)

// RecordGrantAccessStart records the start of a grant access operation
func RecordGrantAccessStart(ctx context.Context, bg *accessv1alpha1.Breakglass) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, OperationNameGrantAccess)
	span.SetAttributes(
		attribute.String(AttributeKeyBreakglassName, bg.Name),
		attribute.String(AttributeKeyRequestNamespace, bg.Spec.Namespace),
		attribute.String(AttributeKeyBreakglassRole, bg.Spec.Role),
		attribute.Int(AttributeKeyBreakglassDurationMinutes, bg.Spec.DurationMinutes),
	)
	return ctx, span
}

// RecordRevokeAccessStart records the start of a revoke access operation
func RecordRevokeAccessStart(ctx context.Context, bg *accessv1alpha1.Breakglass) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, OperationNameRevokeAccess)
	span.SetAttributes(
		attribute.String(AttributeKeyBreakglassName, bg.Name),
		attribute.String(AttributeKeyRequestNamespace, bg.Spec.Namespace),
		attribute.String(AttributeKeyBreakglassRole, bg.Spec.Role),
	)
	return ctx, span
}

// RecordSubjectResolution records subject resolution with telemetry
func RecordSubjectResolution(ctx context.Context, bg *accessv1alpha1.Breakglass, subjectName string) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String(AttributeKeySubject, subjectName))
}

// RecordGrantAccessSuccess records successful grant access with all metrics
func RecordGrantAccessSuccess(bg *accessv1alpha1.Breakglass, subjectName string) {
	RecordBreakglassGranted(
		bg.Spec.Namespace,
		bg.Spec.Role,
		bg.Spec.DurationMinutes,
		bg.Status.ApprovedBy,
	)
	RecordRoleBindingOperation(
		LabelResultSuccess,
		LabelOperationCreate,
		bg.Spec.Role,
		bg.Spec.Namespace,
	)
}

// RecordGrantAccessFailure records failed grant access with all metrics
func RecordGrantAccessFailure(bg *accessv1alpha1.Breakglass, reason string) {
	RecordBreakglassDenied(
		bg.Spec.Namespace,
		bg.Spec.Role,
		reason,
		constants.ControllerIdentity,
	)
	RecordRoleBindingOperation(
		LabelResultError,
		LabelOperationCreate,
		bg.Spec.Role,
		bg.Spec.Namespace,
	)
	RecordError(
		LabelComponentController,
		LabelErrorTypeAuthorization,
		LabelOperationCreate,
	)
}

// RecordGrantAccessValidationFailure records validation failure with all metrics
func RecordGrantAccessValidationFailure(bg *accessv1alpha1.Breakglass) {
	RecordBreakglassDenied(
		bg.Spec.Namespace,
		bg.Spec.Role,
		LabelReasonValidation,
		constants.ControllerIdentity,
	)
	RecordValidationOperation(
		LabelResultError,
		LabelComponentController,
		LabelErrorTypeValidation,
	)
}

// RecordRevokeAccessSuccess records successful revoke access with all metrics
func RecordRevokeAccessSuccess(bg *accessv1alpha1.Breakglass) {
	RecordBreakglassExpired(bg.Spec.Namespace, bg.Spec.Role)
	RecordRoleBindingOperation(
		LabelResultSuccess,
		LabelOperationDelete,
		bg.Spec.Role,
		bg.Spec.Namespace,
	)
}

// RecordRevokeAccessFailure records failed revoke access with all metrics
func RecordRevokeAccessFailure(bg *accessv1alpha1.Breakglass) {
	RecordRoleBindingOperation(
		LabelResultError,
		LabelOperationDelete,
		bg.Spec.Role,
		bg.Spec.Namespace,
	)
	RecordError(
		LabelComponentController,
		LabelErrorTypeAuthorization,
		LabelOperationDelete,
	)
}

// RecordStatusUpdateError records status update errors
func RecordStatusUpdateError(operation string) {
	RecordError(
		LabelComponentController,
		LabelErrorTypeInternal,
		operation,
	)
}

// RecordReconciliationNotFound records when a breakglass resource is not found
func RecordReconciliationNotFound(namespace string) {
	RecordReconcileResult(LabelResultSuccess, string(ReconciliationResultNotFound), namespace)
}

// RecordReconciliationError records when a reconciliation encounters an error
func RecordReconciliationError(namespace string) {
	RecordReconcileResult(LabelResultError, string(ReconciliationResultError), namespace)
}

// RecordReconciliationNoAction records when no action is taken during reconciliation
func RecordReconciliationNoAction(namespace string) {
	RecordReconcileResult(LabelResultSuccess, string(ReconciliationResultNoAction), namespace)
}

// RecordReconciliationExpired records when a breakglass is expired during reconciliation
func RecordReconciliationExpired(namespace string, success bool) {
	result := LabelResultSuccess
	if !success {
		result = LabelResultError
	}
	RecordReconcileResult(result, string(ReconciliationPhaseExpired), namespace)
}

// RecordReconciliationActive records when a breakglass is activated during reconciliation
func RecordReconciliationActive(namespace string, success bool) {
	result := LabelResultSuccess
	if !success {
		result = LabelResultError
	}
	RecordReconcileResult(result, string(ReconciliationPhaseActive), namespace)
}
