package handlers

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
)

// ApprovedCondition handles breakglass requests in the approved condition
type ApprovedCondition struct {
	handler *Handler
}

// NewApprovedCondition creates a new ApprovedCondition
func NewApprovedCondition(handler *Handler) *ApprovedCondition {
	return &ApprovedCondition{handler: handler}
}

// Handle processes a breakglass request in the approved state
func (h *ApprovedCondition) Handle(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	if bg.Status.ApprovedBy == "" {
		log.V(1).Info("waiting for approval in approved condition")
		if err := h.handler.updateStatus(
			ctx,
			bg,
			accessv1alpha1.ConditionPending,
			accessv1alpha1.ReasonWaitingForApproval,
			"Breakglass request is pending approval",
		); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	log.V(1).Info("approval confirmed, processing schedule")
	return h.handler.RecurringPendingCondition().Handle(ctx, bg)
}
