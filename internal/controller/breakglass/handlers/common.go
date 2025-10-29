/*
Copyright 2024 The Cloud-Nimbus Authors.

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

package handlers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/controller"
	"github.com/cloud-nimbus/firedoor/internal/controller/breakglass/usecases"
	internalerrors "github.com/cloud-nimbus/firedoor/internal/errors"
)

const BreakglassGrantedMsgFmt = "Breakglass access granted by %s"
const DefaultApprover = "system"
const DefaultBackoff = 10 * time.Second

// Handler handles breakglass condition transitions
type Handler struct {
	Client                    client.Client
	Operator                  controller.BreakglassOperator
	RecurringManager          controller.RecurringManager
	Alerts                    controller.AlertService
	Clock                     controller.Clock
	Backoff                   time.Duration
	recorder                  record.EventRecorder
	recurringPendingCondition *RecurringPendingCondition
	recurringActiveCondition  *RecurringActiveCondition
}

// NewHandler creates a new Handler
func NewHandler(
	client client.Client,
	operator controller.BreakglassOperator,
	recurringManager controller.RecurringManager,
	alerts controller.AlertService,
	clock controller.Clock,
	recorder record.EventRecorder,
	backoff ...time.Duration,
) *Handler {
	bo := DefaultBackoff
	if len(backoff) > 0 {
		bo = backoff[0]
	}

	handler := &Handler{
		Client:           client,
		Operator:         operator,
		RecurringManager: recurringManager,
		Alerts:           alerts,
		Clock:            clock,
		recorder:         recorder,
		Backoff:          bo,
	}

	// Initialize recurring condition handlers
	handler.recurringPendingCondition = NewRecurringPendingCondition(handler)
	handler.recurringActiveCondition = NewRecurringActiveCondition(handler)

	return handler
}

// RecurringPendingCondition returns the recurring pending condition handler
func (h *Handler) RecurringPendingCondition() *RecurringPendingCondition {
	return h.recurringPendingCondition
}

// RecurringActiveCondition returns the recurring active condition handler
func (h *Handler) RecurringActiveCondition() *RecurringActiveCondition {
	return h.recurringActiveCondition
}

// updateStatus updates the breakglass status with the given condition and reason using a status patch
func (h *Handler) updateStatus(
	ctx context.Context,
	bg *accessv1alpha1.Breakglass,
	condition accessv1alpha1.BreakglassCondition,
	reason accessv1alpha1.BreakglassConditionReason,
	message string,
) error {
	bg.Status.ObservedGeneration = bg.Generation
	conditionObj := metav1.Condition{
		Type:               string(condition),
		Status:             metav1.ConditionTrue,
		Reason:             string(reason),
		Message:            message,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: bg.Generation,
	}
	meta.SetStatusCondition(&bg.Status.Conditions, conditionObj)

	return h.Client.Status().Update(ctx, bg)

}

// GrantAndActivate grants access, sets ApprovedBy, updates status, and handles backoff.
func (h *Handler) GrantAndActivate(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	previousResources := append([]string(nil), bg.Status.CreatedResources...)
	if err := h.Operator.GrantAccess(ctx, bg); err != nil {
		log.Error(err, "failed to grant access")

		// Check if this is a retryable error
		var rbacErr *internalerrors.RBACError
		if errors.As(err, &rbacErr) && rbacErr.IsRetryable() {
			log.Info("retryable RBAC error, will retry", "operation", rbacErr.Operation, "resource", rbacErr.Resource)
			return ctrl.Result{RequeueAfter: h.Backoff}, nil
		}

		// Emit error event for permanent failures
		h.emitAccessGrantFailedEvent(bg, err)

		// Permanent error - update status to failed
		if err := h.updateStatus(
			ctx,
			bg,
			accessv1alpha1.ConditionFailed,
			accessv1alpha1.ReasonRBACCreationFailed,
			err.Error(),
		); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if len(bg.Status.CreatedResources) > len(previousResources) {
		newResources := append([]string(nil), bg.Status.CreatedResources[len(previousResources):]...)
		log.Info("breakglass permissions created", "resources", newResources)
	}

	// Set ApprovedBy to default (system) if not set
	if bg.Status.ApprovedBy == "" {
		bg.Status.ApprovedBy = DefaultApprover
	}
	// Update status to active
	msg := fmt.Sprintf(BreakglassGrantedMsgFmt, bg.Status.ApprovedBy)

	now := h.Clock.Now()
	bg.Status.GrantedAt = &metav1.Time{Time: now}
	bg.Status.ExpiresAt = nil

	window, hasWindow := usecases.CurrentWindow(bg, now)
	if hasWindow {
		log.Info("calculated activation window", "start", window.Start, "endsAt", window.End)
	} else {
		log.Info("breakglass granted without activation window")
	}

	if h.RecurringManager != nil {
		if err := h.RecurringManager.OnActivationGranted(ctx, bg); err != nil {
			log.Error(err, "failed to advance recurring schedule after grant")
			return ctrl.Result{}, err
		}
	}

	// Update status to active
	if err := h.updateStatus(
		ctx,
		bg,
		accessv1alpha1.ConditionRecurringActive,
		accessv1alpha1.ReasonRecurringActivated,
		msg,
	); err != nil {
		return ctrl.Result{}, err
	}

	// Emit event for successful access grant
	h.emitAccessGrantedEvent(bg)
	// Requeue based on expiration if set
	if hasWindow {
		timeUntil := h.Clock.Until(window.End)
		timeUntil = clampRequeueDuration(timeUntil, 30*time.Second, time.Hour)
		log.V(1).Info("requeuing until expiration", "after", timeUntil)
		return ctrl.Result{RequeueAfter: timeUntil}, nil
	}

	return ctrl.Result{}, nil

}

// RevokeAndExpire revokes access, sets expired condition, and handles backoff.
func (h *Handler) RevokeAndExpire(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("revoking and expiring breakglass access")
	if err := h.Operator.RevokeAccess(ctx, bg); err != nil {
		log.Error(err, "revoke failed")

		// Check if this is a retryable error
		var rbacErr *internalerrors.RBACError
		if errors.As(err, &rbacErr) {
			if rbacErr != nil && rbacErr.IsRetryable() {
				log.Info("retryable RBAC error, will retry", "operation", rbacErr.Operation, "resource", rbacErr.Resource)
				return ctrl.Result{RequeueAfter: h.Backoff}, nil
			}

			// Check if the error is NotFound - if so, we can proceed to completion handling.
			if rbacErr != nil && internalerrors.IsNotFoundError(rbacErr.Err) {
				log.Info("RBAC resources already deleted, proceeding with completion state")
				return h.postRevokeTransition(ctx, bg)
			}
		}

		op := "unknown"
		resource := "unknown"
		if rbacErr != nil {
			op = rbacErr.Operation
			resource = rbacErr.Resource
		}

		log.Info("permanent RBAC error, marking as failed", "operation", op, "resource", resource)
		// Emit error event for permanent failures
		h.emitAccessRevokeFailedEvent(bg, err)
		_ = h.updateStatus(ctx, bg,
			accessv1alpha1.ConditionFailed,
			accessv1alpha1.ReasonRevokeFailed,
			err.Error(),
		)
		// Permanent error - return it to trigger reconciliation failure
		return ctrl.Result{}, err
	}

	return h.postRevokeTransition(ctx, bg)
}

func (h *Handler) postRevokeTransition(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	if usecases.HasFutureActivations(bg) {
		return h.transitionToRecurringPending(ctx, bg)
	}
	return h.markExpired(ctx, bg)
}

func (h *Handler) transitionToRecurringPending(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	bg.Status.ExpiresAt = nil

	reason := accessv1alpha1.ReasonRecurringWaiting
	message := "Recurring breakglass pending next activation"
	if bg.Status.NextActivationAt != nil {
		next := bg.Status.NextActivationAt.Time
		reason = accessv1alpha1.ReasonRecurringScheduled
		message = fmt.Sprintf(
			"Recurring breakglass scheduled for next activation at %s",
			next.Format(time.RFC3339),
		)
	}

	if err := h.updateStatus(
		ctx,
		bg,
		accessv1alpha1.ConditionRecurringPending,
		reason,
		message,
	); err != nil {
		return ctrl.Result{}, err
	}

	h.emitAccessRevokedEvent(bg)

	requeue := 30 * time.Second
	if bg.Status.NextActivationAt != nil {
		requeue = clampRequeueDuration(h.Clock.Until(bg.Status.NextActivationAt.Time), 30*time.Second, time.Hour)
	}

	return ctrl.Result{RequeueAfter: requeue}, nil
}

// emitAccessGrantedEvent emits a Kubernetes event when access is successfully granted
func (h *Handler) emitAccessGrantedEvent(bg *accessv1alpha1.Breakglass) {
	if h.recorder == nil {
		return
	}

	clusterRoleCount := len(bg.Spec.ClusterRoles)
	policyCount := len(bg.Spec.Policy)

	h.recorder.Eventf(bg, "Normal", "AccessGranted",
		"Created %d ClusterRoleBindings and %d RoleBindings", clusterRoleCount, policyCount)
}

// emitAccessRevokedEvent emits a Kubernetes event when access is successfully revoked
func (h *Handler) emitAccessRevokedEvent(bg *accessv1alpha1.Breakglass) {
	if h.recorder == nil {
		return
	}

	h.recorder.Eventf(bg, "Normal", "AccessRevoked",
		"Revoked breakglass access and cleaned up RBAC resources")
}

// emitErrorEvent emits a Kubernetes error event with the given reason and message
func (h *Handler) emitErrorEvent(bg *accessv1alpha1.Breakglass, reason, msgFmt string, args ...any) {
	if h.recorder == nil {
		return
	}

	h.recorder.Eventf(bg, "Warning", reason, msgFmt, args...)
}

// emitAccessGrantFailedEvent emits a Kubernetes event when access grant fails
func (h *Handler) emitAccessGrantFailedEvent(bg *accessv1alpha1.Breakglass, err error) {
	h.emitErrorEvent(bg, "AccessGrantFailed", "Failed to grant breakglass access: %v", err)
}

// emitAccessRevokeFailedEvent emits a Kubernetes event when access revocation fails
func (h *Handler) emitAccessRevokeFailedEvent(bg *accessv1alpha1.Breakglass, err error) {
	h.emitErrorEvent(bg, "AccessRevokeFailed", "Failed to revoke breakglass access: %v", err)
}

func clampRequeueDuration(d, min, max time.Duration) time.Duration {
	if d < min {
		return min
	}
	if d > max {
		return max
	}
	return d
}

func (h *Handler) markExpired(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	if final, ok := usecases.FinalCompletionTime(bg, h.Clock.Now()); ok {
		bg.Status.ExpiresAt = &metav1.Time{Time: *final}
	} else {
		bg.Status.ExpiresAt = nil
	}
	bg.Status.NextActivationAt = nil

	if err := h.updateStatus(
		ctx,
		bg,
		accessv1alpha1.ConditionExpired,
		accessv1alpha1.ReasonAccessExpired,
		"Breakglass access expired",
	); err != nil {
		return ctrl.Result{}, err
	}
	h.emitAccessRevokedEvent(bg)
	return ctrl.Result{}, nil
}
