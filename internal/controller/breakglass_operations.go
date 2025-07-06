package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/conditions"
	"github.com/cloud-nimbus/firedoor/internal/constants"
	"github.com/cloud-nimbus/firedoor/internal/telemetry"
)

// BreakglassOperator defines the interface for breakglass operations.
type BreakglassOperator interface {
	GrantAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error)
	RevokeAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error)
}

// breakglassOperator implements BreakglassOperator.
type breakglassOperator struct {
	client   client.Client
	recorder record.EventRecorder
}

// NewBreakglassOperator creates a new breakglass operator.
func NewBreakglassOperator(client client.Client, recorder record.EventRecorder) BreakglassOperator {
	return &breakglassOperator{
		client:   client,
		recorder: recorder,
	}
}

// GrantAccess grants access to a breakglass resource.
func (o *breakglassOperator) GrantAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	ctx, span := telemetry.RecordGrantAccessStart(ctx, bg)
	defer span.End()

	subject, err := o.resolveSubjectAndHandleError(ctx, bg)
	if err != nil {
		return ctrl.Result{}, nil
	}

	telemetry.RecordSubjectResolution(ctx, bg, subject.Name)

	if err := o.createRoleBinding(ctx, bg, subject); err != nil {
		return ctrl.Result{}, err
	}

	return o.completeGrantAccess(ctx, bg, subject)
}

// resolveSubjectAndHandleError resolves the subject and handles validation errors
func (o *breakglassOperator) resolveSubjectAndHandleError(ctx context.Context, bg *accessv1alpha1.Breakglass) (*rbacv1.Subject, error) {
	subject, err := resolveSubject(bg)
	if err != nil {
		logger := log.FromContext(ctx)
		logger.Info("Breakglass missing user or group; skipping grant")

		telemetry.RecordGrantAccessValidationFailure(bg)
		o.setDeniedStatus(ctx, bg, err)
		o.emitEvent(bg, corev1.EventTypeWarning, "InvalidRequest", "Missing user or group")

		return nil, err
	}
	return subject, nil
}

// setDeniedStatus sets the denied status and conditions
func (o *breakglassOperator) setDeniedStatus(ctx context.Context, bg *accessv1alpha1.Breakglass, err error) {
	phase := accessv1alpha1.PhaseDenied
	bg.Status.Phase = &phase
	bg.Status.ApprovedBy = constants.ControllerIdentity

	bg.Status.Conditions = append(bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Denied.String(),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             conditions.InvalidRequest.String(),
		Message:            fmt.Sprintf("Missing user or group: %v", err),
	})
	bg.Status.Conditions = append(bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Approved.String(),
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             conditions.InvalidRequest.String(),
		Message:            conditions.RequestDeniedDueToMissingUserOrGroup.String(),
	})

	if err := o.client.Status().Update(ctx, bg); err != nil {
		telemetry.RecordStatusUpdateError("update")
	}
}

// createRoleBinding creates the role binding for the breakglass
func (o *breakglassOperator) createRoleBinding(ctx context.Context, bg *accessv1alpha1.Breakglass, subject *rbacv1.Subject) error {
	roleBinding := o.buildRoleBinding(bg, subject)

	if err := o.client.Create(ctx, roleBinding); err != nil && !apierrors.IsAlreadyExists(err) {
		telemetry.RecordGrantAccessFailure(bg, "role_binding_failed")
		o.setRoleBindingFailureStatus(ctx, bg, err)
		return err
	}

	telemetry.RecordRoleBindingOperation(
		telemetry.LabelResultSuccess,
		telemetry.LabelOperationCreate,
		bg.Spec.Role,
		bg.Spec.Namespace,
	)
	return nil
}

// buildRoleBinding builds the role binding object
func (o *breakglassOperator) buildRoleBinding(bg *accessv1alpha1.Breakglass, subject *rbacv1.Subject) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("breakglass-%s", bg.Name),
			Namespace: bg.Spec.Namespace,
		},
		Subjects: []rbacv1.Subject{*subject},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     bg.Spec.Role,
			APIGroup: rbacv1.GroupName,
		},
	}
}

// setRoleBindingFailureStatus sets the status for role binding failures
func (o *breakglassOperator) setRoleBindingFailureStatus(ctx context.Context, bg *accessv1alpha1.Breakglass, err error) {
	bg.Status.Conditions = append(bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Denied.String(),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             conditions.RoleBindingFailed.String(),
		Message:            fmt.Sprintf("Failed to create RoleBinding: %v", err),
	})
	bg.Status.Conditions = append(bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Approved.String(),
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             conditions.RoleBindingFailed.String(),
		Message:            conditions.AccessDeniedDueToRoleBindingFailure.String(),
	})

	if updateErr := o.client.Status().Update(ctx, bg); updateErr != nil {
		telemetry.RecordStatusUpdateError("update")
	}
}

// completeGrantAccess completes the grant access operation
func (o *breakglassOperator) completeGrantAccess(ctx context.Context, bg *accessv1alpha1.Breakglass, subject *rbacv1.Subject) (ctrl.Result, error) {
	now := metav1.Now()
	expiry := metav1.NewTime(now.Time.Add(time.Duration(bg.Spec.DurationMinutes) * time.Minute))
	phase := accessv1alpha1.PhaseActive

	telemetry.RecordGrantAccessSuccess(bg, subject.Name)

	o.updateGrantedStatus(bg, &phase, &now, &expiry)
	o.setGrantedConditions(bg, subject, expiry)

	if err := o.client.Status().Update(ctx, bg); err != nil {
		telemetry.RecordStatusUpdateError("update")
		return ctrl.Result{}, err
	}

	log := log.FromContext(ctx)
	log.Info("Granted breakglass access", "subject", subject.Name, "expiresAt", expiry)
	return ctrl.Result{RequeueAfter: time.Until(expiry.Time)}, nil
}

// updateGrantedStatus updates the status with granted information
func (o *breakglassOperator) updateGrantedStatus(bg *accessv1alpha1.Breakglass, phase *accessv1alpha1.BreakglassPhase, now, expiry *metav1.Time) {
	bg.Status.Phase = phase
	bg.Status.GrantedAt = now
	bg.Status.ExpiresAt = expiry
}

// setGrantedConditions sets the conditions for granted access
func (o *breakglassOperator) setGrantedConditions(bg *accessv1alpha1.Breakglass, subject *rbacv1.Subject, expiry metav1.Time) {
	bg.Status.Conditions = append(bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Approved.String(),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             conditions.AccessGranted.String(),
		Message:            fmt.Sprintf("Breakglass access granted to %s until %s", subject.Name, expiry.Format(time.RFC3339)),
	})
	bg.Status.Conditions = append(bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Active.String(),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             conditions.AccessActive.String(),
		Message:            fmt.Sprintf("Breakglass access is active until %s", expiry.Format(time.RFC3339)),
	})
	bg.Status.Conditions = append(bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Denied.String(),
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             conditions.AccessGranted.String(),
		Message:            "Access has been granted",
	})
}

// RevokeAccess revokes access from a breakglass resource.
func (o *breakglassOperator) RevokeAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	ctx, span := telemetry.RecordRevokeAccessStart(ctx, bg)
	defer span.End()

	if err := o.deleteRoleBinding(ctx, bg); err != nil {
		return ctrl.Result{}, err
	}

	return o.completeRevokeAccess(ctx, bg)
}

// deleteRoleBinding deletes the role binding
func (o *breakglassOperator) deleteRoleBinding(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("breakglass-%s", bg.Name),
			Namespace: bg.Spec.Namespace,
		},
	}

	if err := o.client.Delete(ctx, roleBinding); err != nil && !apierrors.IsNotFound(err) {
		telemetry.RecordRevokeAccessFailure(bg)
		o.setRevokeFailureStatus(ctx, bg, err)
		return err
	}

	telemetry.RecordRevokeAccessSuccess(bg)
	return nil
}

// setRevokeFailureStatus sets the status for revoke failures
func (o *breakglassOperator) setRevokeFailureStatus(ctx context.Context, bg *accessv1alpha1.Breakglass, err error) {
	bg.Status.Conditions = append(bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Expired.String(),
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             conditions.RevokeFailed.String(),
		Message:            fmt.Sprintf("Failed to delete RoleBinding: %v", err),
	})

	if updateErr := o.client.Status().Update(ctx, bg); updateErr != nil {
		telemetry.RecordStatusUpdateError("update")
	}
}

// completeRevokeAccess completes the revoke access operation
func (o *breakglassOperator) completeRevokeAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	phase := accessv1alpha1.PhaseExpired
	bg.Status.Phase = &phase

	o.setRevokedConditions(bg)

	if err := o.client.Status().Update(ctx, bg); err != nil {
		telemetry.RecordStatusUpdateError("update")
		return ctrl.Result{}, err
	}

	logger := log.FromContext(ctx)
	logger.Info("Revoked breakglass access")
	return ctrl.Result{}, nil
}

// setRevokedConditions sets the conditions for revoked access
func (o *breakglassOperator) setRevokedConditions(bg *accessv1alpha1.Breakglass) {
	bg.Status.Conditions = append(bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Expired.String(),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             conditions.AccessExpired.String(),
		Message:            conditions.BreakglassAccessExpiredAndRevoked.String(),
	})
	bg.Status.Conditions = append(bg.Status.Conditions, metav1.Condition{
		Type:               conditions.Active.String(),
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             conditions.AccessExpired.String(),
		Message:            conditions.AccessIsNoLongerActive.String(),
	})
}

// emitEvent emits an event for the breakglass
func (o *breakglassOperator) emitEvent(bg *accessv1alpha1.Breakglass, eventType, reason, message string) {
	o.recorder.Event(bg, eventType, reason, message)
}
