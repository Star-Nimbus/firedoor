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

package controller

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/alerting"
	"github.com/cloud-nimbus/firedoor/internal/conditions"
	"github.com/cloud-nimbus/firedoor/internal/config"
	"github.com/cloud-nimbus/firedoor/internal/telemetry"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

var tracer = otel.Tracer("firedoor/internal/controller/breakglass")

const (
	breakglassFinalizer = "breakglass.firedoor.cloudnimbus.io/finalizer"
	AutoApprover        = "system-auto-approve"
)

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// remove removes a string from a slice
func remove(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// BreakglassReconciler reconciles a Breakglass object
type BreakglassReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Recorder         record.EventRecorder
	Config           *config.Config
	operator         BreakglassOperator
	recurringManager *RecurringBreakglassManager
	alertService     *alerting.AlertmanagerService
}

// +kubebuilder:rbac:groups=access.cloudnimbus.io,resources=breakglasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=access.cloudnimbus.io,resources=breakglasses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=access.cloudnimbus.io,resources=breakglasses/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;create;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BreakglassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, span := tracer.Start(ctx, "breakglass.reconcile",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String(telemetry.AttributeKeyNamespace.String(), req.Namespace),
			attribute.String(telemetry.AttributeKeyBreakglassName.String(), req.Name),
		),
	)
	defer span.End()

	// Start metrics timer for reconciliation duration
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		telemetry.ObserveReconcileDurationSecondsWithExemplar(duration, trace.SpanFromContext(ctx))
	}()

	bg := &accessv1alpha1.Breakglass{}
	if err := r.Client.Get(ctx, req.NamespacedName, bg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if client.IgnoreNotFound(err) == nil {
			telemetry.RecordReconciliationNotFound(req.Namespace)
		} else {
			telemetry.RecordReconciliationError(req.Namespace)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle finalizer for cleanup of cross-namespace resources
	if bg.DeletionTimestamp != nil {
		// Mark telemetry for finalizer flow
		telemetry.RecordOperation(telemetry.OpDelete, telemetry.ResultSuccess, telemetry.ComponentController, "finalizer", req.Namespace)
		return r.handleFinalizer(ctx, req.NamespacedName)
	}

	// Record creation metric if this is a new breakglass
	if bg.Status.Phase == "" {
		telemetry.RecordOperation(telemetry.OpCreate, telemetry.ResultSuccess, telemetry.ComponentController, telemetry.RoleUnknown, req.Namespace)
	}

	// Handle different phases
	switch bg.Status.Phase {
	case "":
		// New breakglass - set to Pending or RecurringPending
		return r.handleNewBreakglass(ctx, bg)
	case accessv1alpha1.PhasePending:
		return r.handlePendingBreakglass(ctx, bg)
	case accessv1alpha1.PhaseActive:
		return r.handleActiveBreakglass(ctx, bg)
	case accessv1alpha1.PhaseRecurringPending:
		return r.handleRecurringPendingBreakglass(ctx, bg)
	case accessv1alpha1.PhaseRecurringActive:
		return r.handleRecurringActiveBreakglass(ctx, bg)
	case accessv1alpha1.PhaseExpired, accessv1alpha1.PhaseDenied, accessv1alpha1.PhaseRevoked:
		// Final states - no further action needed
		telemetry.RecordReconciliationNoAction(req.Namespace)
		return ctrl.Result{}, nil
	default:
		// Unknown phase - log and return
		telemetry.RecordReconciliationError(req.Namespace)
		return ctrl.Result{}, fmt.Errorf("unknown breakglass phase: %s", bg.Status.Phase)
	}
}

// approved checks if the breakglass is approved (either manually or auto-approved)
func approved(bg *accessv1alpha1.Breakglass) bool {
	return bg.Status.ApprovedBy != ""
}

// isExpired checks if the breakglass has expired (works for both Active and RecurringActive phases)
func isExpired(bg *accessv1alpha1.Breakglass) bool {
	return (bg.Status.Phase == accessv1alpha1.PhaseActive || bg.Status.Phase == accessv1alpha1.PhaseRecurringActive) &&
		bg.Status.ExpiresAt != nil &&
		time.Now().After(bg.Status.ExpiresAt.Time)
}

// handleNewBreakglass handles a newly created breakglass
func (r *BreakglassReconciler) handleNewBreakglass(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(bg, breakglassFinalizer) {
		controllerutil.AddFinalizer(bg, breakglassFinalizer)
		if err := retry.OnError(retry.DefaultRetry, apierrors.IsConflict, func() error {
			return r.Client.Update(ctx, bg)
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	// Set initial phase based on whether it's recurring or not
	if bg.Spec.Recurring {
		bg.Status.Phase = accessv1alpha1.PhaseRecurringPending
		// Initialize recurring status
		if err := r.recurringManager.TransitionToRecurringPending(bg); err != nil {
			return ctrl.Result{}, err
		}
		// Set recurring pending condition
		meta.SetStatusCondition(&bg.Status.Conditions, metav1.Condition{
			Type:               conditions.RecurringPending.String(),
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             conditions.RecurringAccessScheduled.String(),
			Message:            conditions.RecurringAccessScheduledForActivation.String(),
		})
	} else {
		bg.Status.Phase = accessv1alpha1.PhasePending
	}
	bg.Status.ApprovedBy = ""
	bg.Status.ApprovedAt = nil

	// If approval is not required, auto-approve
	if !bg.Spec.ApprovalRequired {
		now := metav1.Now()
		bg.Status.ApprovedBy = AutoApprover
		bg.Status.ApprovedAt = &now
	}

	// Single status update with all changes
	if err := retry.OnError(retry.DefaultRetry, apierrors.IsConflict, func() error {
		return r.Client.Status().Update(ctx, bg)
	}); err != nil {
		return ctrl.Result{}, err
	}

	// If approval is not required, handle based on whether it's recurring or not
	if !bg.Spec.ApprovalRequired {
		if bg.Spec.Recurring {
			return r.handleRecurringPendingBreakglass(ctx, bg)
		} else {
			return r.handlePendingBreakglass(ctx, bg)
		}
	}

	// If approval is required, wait for manual approval
	return ctrl.Result{}, nil
}

// handlePendingBreakglass handles a breakglass in Pending phase
func (r *BreakglassReconciler) handlePendingBreakglass(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	// If approval is required, wait for manual approval
	if bg.Spec.ApprovalRequired {
		if !approved(bg) {
			// Still waiting for approval - add jitter to avoid thundering herd
			jitter := time.Duration(rand.Intn(30)) * time.Second
			return ctrl.Result{RequeueAfter: 30*time.Second + jitter}, nil
		}
	}

	// If already denied, do nothing
	if bg.Status.Phase == accessv1alpha1.PhaseDenied {
		return ctrl.Result{}, nil
	}

	// If already active, do nothing
	if bg.Status.Phase == accessv1alpha1.PhaseActive {
		return ctrl.Result{}, nil
	}

	// Grant access and transition to Active
	nsKey := telemetry.NamespaceKey(bg)
	result, err := r.operator.GrantAccess(ctx, bg)
	if err != nil {
		span := trace.SpanFromContext(ctx)
		span.SetStatus(codes.Error, err.Error())
		telemetry.RecordReconciliationActive(nsKey, false)
		return result, err
	}

	telemetry.RecordReconciliationActive(nsKey, true)
	// Use the operator's requeue time to avoid hot loops
	return result, nil
}

// handleActiveBreakglass handles a breakglass in Active phase
func (r *BreakglassReconciler) handleActiveBreakglass(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	// Check if expired
	if isExpired(bg) {
		nsKey := telemetry.NamespaceKey(bg)
		result, err := r.operator.RevokeAccess(ctx, bg)
		if err != nil {
			span := trace.SpanFromContext(ctx)
			span.SetStatus(codes.Error, err.Error())
			telemetry.RecordReconciliationExpired(nsKey, false)
		} else {
			telemetry.RecordReconciliationExpired(nsKey, true)
		}
		return result, err
	}

	// Still active - requeue when it expires
	if bg.Status.ExpiresAt != nil {
		return ctrl.Result{RequeueAfter: time.Until(bg.Status.ExpiresAt.Time)}, nil
	}

	return ctrl.Result{}, nil
}

// resolveSubjects converts SubjectRefs to RBAC subjects
func resolveSubjects(ctx context.Context, bg *accessv1alpha1.Breakglass) ([]rbacv1.Subject, error) {
	ctx, span := tracer.Start(ctx, "subjectResolution")
	defer span.End()

	if len(bg.Spec.Subjects) == 0 {
		return nil, fmt.Errorf("no subjects provided")
	}

	subjects := make([]rbacv1.Subject, 0, len(bg.Spec.Subjects))
	for _, subjectRef := range bg.Spec.Subjects {
		subject := rbacv1.Subject{
			Kind: subjectRef.Kind,
			Name: subjectRef.Name,
		}

		// Set APIGroup for User subjects
		if subjectRef.Kind == rbacv1.UserKind {
			subject.APIGroup = rbacv1.GroupName
		}

		// Set namespace for ServiceAccount subjects
		if subjectRef.Kind == rbacv1.ServiceAccountKind {
			subject.Namespace = subjectRef.Namespace
		}

		subjects = append(subjects, subject)
	}

	return subjects, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BreakglassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a default config if none is provided
	if r.Config == nil {
		r.Config = config.NewDefaultConfig()
	}

	r.alertService = alerting.NewAlertmanagerService(&r.Config.Alertmanager, r.Recorder)
	r.operator = NewBreakglassOperator(r.Client, r.Recorder, r.alertService, r.Config.Controller.PrivilegeEscalation)
	r.recurringManager = NewRecurringBreakglassManager(time.UTC)
	return ctrl.NewControllerManagedBy(mgr).
		For(&accessv1alpha1.Breakglass{}).
		Complete(r)
}

// ApproveBreakglass approves a pending breakglass request
func (r *BreakglassReconciler) ApproveBreakglass(ctx context.Context, bg *accessv1alpha1.Breakglass, approver string) error {
	if bg.Status.Phase != accessv1alpha1.PhasePending {
		return fmt.Errorf("cannot approve breakglass in phase %s", bg.Status.Phase)
	}

	if approved(bg) {
		return fmt.Errorf("breakglass already approved by %s", bg.Status.ApprovedBy)
	}

	now := metav1.Now()
	bg.Status.ApprovedBy = approver
	bg.Status.ApprovedAt = &now

	// Add approval condition
	meta.SetStatusCondition(&bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Approved.String(),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             conditions.AccessGranted.String(),
		Message:            fmt.Sprintf("Approved by %s", approver),
	})

	if err := retry.OnError(retry.DefaultRetry, apierrors.IsConflict, func() error {
		return r.Client.Status().Update(ctx, bg)
	}); err != nil {
		return fmt.Errorf("failed to update breakglass status: %w", err)
	}

	r.Recorder.Event(bg, "Normal", "Approved", fmt.Sprintf("Breakglass approved by %s", approver))
	return nil
}

// DenyBreakglass denies a pending breakglass request
func (r *BreakglassReconciler) DenyBreakglass(ctx context.Context, bg *accessv1alpha1.Breakglass, denier string, reason string) error {
	if bg.Status.Phase != accessv1alpha1.PhasePending {
		return fmt.Errorf("cannot deny breakglass in phase %s", bg.Status.Phase)
	}

	if approved(bg) {
		return fmt.Errorf("breakglass already approved by %s", bg.Status.ApprovedBy)
	}

	now := metav1.Now()
	bg.Status.Phase = accessv1alpha1.PhaseDenied

	// Add denial condition
	meta.SetStatusCondition(&bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Denied.String(),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             conditions.AccessDenied.String(),
		Message:            fmt.Sprintf("Denied by %s: %s", denier, reason),
	})

	if err := retry.OnError(retry.DefaultRetry, apierrors.IsConflict, func() error {
		return r.Client.Status().Update(ctx, bg)
	}); err != nil {
		return fmt.Errorf("failed to update breakglass status: %w", err)
	}

	r.Recorder.Event(bg, "Warning", "Denied", fmt.Sprintf("Breakglass denied by %s: %s", denier, reason))
	return nil
}

// RevokeBreakglass revokes an active breakglass request
func (r *BreakglassReconciler) RevokeBreakglass(ctx context.Context, bg *accessv1alpha1.Breakglass, revoker string, reason string) error {
	if bg.Status.Phase != accessv1alpha1.PhaseActive {
		return fmt.Errorf("cannot revoke breakglass in phase %s", bg.Status.Phase)
	}

	// Revoke access through operator
	_, err := r.operator.RevokeAccess(ctx, bg)
	if err != nil {
		return fmt.Errorf("failed to revoke access: %w", err)
	}

	// Update status to revoked
	now := metav1.Now()
	bg.Status.Phase = accessv1alpha1.PhaseRevoked

	// Add revocation condition
	meta.SetStatusCondition(&bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Revoked.String(),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             conditions.AccessRevoked.String(),
		Message:            fmt.Sprintf("Revoked by %s: %s", revoker, reason),
	})

	if err := retry.OnError(retry.DefaultRetry, apierrors.IsConflict, func() error {
		return r.Client.Status().Update(ctx, bg)
	}); err != nil {
		return fmt.Errorf("failed to update breakglass status: %w", err)
	}

	r.Recorder.Event(bg, "Warning", "Revoked", fmt.Sprintf("Breakglass revoked by %s: %s", revoker, reason))
	return nil
}

// handleRecurringPendingBreakglass handles a recurring breakglass in RecurringPending phase
func (r *BreakglassReconciler) handleRecurringPendingBreakglass(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	// If approval is required, wait for manual approval
	if bg.Spec.ApprovalRequired {
		// Check if approved (this would be set by an approval webhook or manual update)
		if !approved(bg) {
			// Still waiting for approval - requeue based on next activation time
			requeueTime, err := r.recurringManager.GetRequeueTimeForRecurring(bg)
			if err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
			return ctrl.Result{RequeueAfter: requeueTime}, nil
		}
	}

	// Check if it's time to activate
	if r.recurringManager.ShouldActivateRecurring(bg) {
		// Transition to RecurringActive and grant access
		if err := r.recurringManager.TransitionToRecurringActive(bg); err != nil {
			nsKey := telemetry.NamespaceKey(bg)
			telemetry.RecordReconciliationActive(nsKey, false)
			return ctrl.Result{}, err
		}

		// Set recurring active condition
		meta.SetStatusCondition(&bg.Status.Conditions, metav1.Condition{
			Type:               conditions.RecurringActive.String(),
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             conditions.RecurringAccessActive.String(),
			Message:            conditions.RecurringAccessCurrentlyActive.String(),
		})

		// Grant access
		nsKey := telemetry.NamespaceKey(bg)
		result, err := r.operator.GrantAccess(ctx, bg)
		if err != nil {
			span := trace.SpanFromContext(ctx)
			span.SetStatus(codes.Error, err.Error())
			telemetry.RecordReconciliationActive(nsKey, false)
		} else {
			telemetry.RecordReconciliationActive(nsKey, true)
			telemetry.RecordRecurringBreakglassActivationWithTelemetry(nsKey, bg.Status.ActivationCount)
		}
		return result, err
	}

	// Not time to activate yet - requeue based on next activation time
	requeueTime, err := r.recurringManager.GetRequeueTimeForRecurring(bg)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	return ctrl.Result{RequeueAfter: requeueTime}, nil
}

// handleRecurringActiveBreakglass handles a recurring breakglass in RecurringActive phase
func (r *BreakglassReconciler) handleRecurringActiveBreakglass(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	// Check if expired
	if isExpired(bg) {
		nsKey := telemetry.NamespaceKey(bg)
		result, err := r.operator.RevokeAccess(ctx, bg)
		if err != nil {
			span := trace.SpanFromContext(ctx)
			span.SetStatus(codes.Error, err.Error())
			telemetry.RecordReconciliationExpired(nsKey, false)
		} else {
			telemetry.RecordReconciliationExpired(nsKey, true)
			telemetry.RecordRecurringBreakglassExpirationWithTelemetry(nsKey, bg.Status.ActivationCount)
		}

		// Update recurring status and transition back to RecurringPending
		if err := r.recurringManager.UpdateRecurringStatus(bg); err != nil {
			return result, err
		}

		if err := r.recurringManager.TransitionToRecurringPending(bg); err != nil {
			return result, err
		}

		if err := retry.OnError(retry.DefaultRetry, apierrors.IsConflict, func() error {
			return r.Client.Status().Update(ctx, bg)
		}); err != nil {
			return result, err
		}

		return result, err
	}

	// Still active - requeue when it expires
	if bg.Status.ExpiresAt != nil {
		return ctrl.Result{RequeueAfter: time.Until(bg.Status.ExpiresAt.Time)}, nil
	}

	return ctrl.Result{}, nil
}

// handleFinalizer handles the finalizer for cleanup of cross-namespace resources
func (r *BreakglassReconciler) handleFinalizer(ctx context.Context, key client.ObjectKey) (ctrl.Result, error) {
	return ctrl.Result{}, retry.OnError(retry.DefaultRetry, apierrors.IsConflict, func() error {
		// ➊ fresh copy each attempt
		var bg accessv1alpha1.Breakglass
		if err := r.Client.Get(ctx, key, &bg); err != nil {
			return client.IgnoreNotFound(err)
		}

		if bg.DeletionTimestamp.IsZero() || !contains(bg.Finalizers, breakglassFinalizer) {
			return nil // nothing to do
		}

		// ➋ revoke only if still Active
		if bg.Status.Phase == accessv1alpha1.PhaseActive || bg.Status.Phase == accessv1alpha1.PhaseRecurringActive {
			if _, err := r.operator.RevokeAccess(ctx, &bg); err != nil && !apierrors.IsNotFound(err) { // tolerate already-gone RBAC
				return err
			}
		}

		// ➌ remove finalizer
		bg.Finalizers = remove(bg.Finalizers, breakglassFinalizer)
		return r.Client.Update(ctx, &bg)
	})
}
