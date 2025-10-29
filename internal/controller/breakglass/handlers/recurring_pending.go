package handlers

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
)

// RecurringPendingCondition handles breakglass requests in the recurring pending condition
type RecurringPendingCondition struct {
	handler *Handler
}

// NewRecurringPendingCondition creates a new RecurringPendingCondition
func NewRecurringPendingCondition(handler *Handler) *RecurringPendingCondition {
	return &RecurringPendingCondition{handler: handler}
}

// Handle processes a recurring pending breakglass request
func (h *RecurringPendingCondition) Handle(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Set RecurringPending condition if not already set
	if len(bg.Status.Conditions) == 0 || bg.Status.Conditions[len(bg.Status.Conditions)-1].Type != string(accessv1alpha1.ConditionRecurringPending) {
		if err := h.handler.updateStatus(
			ctx,
			bg,
			accessv1alpha1.ConditionRecurringPending,
			accessv1alpha1.ReasonRecurringWaiting,
			"Breakglass is recurring and pending next activation",
		); err != nil {
			return ctrl.Result{}, err
		}
		// Requeue to ensure the new status is picked up
		return ctrl.Result{Requeue: true}, nil
	}

	if err := h.handler.RecurringManager.ProcessRecurring(ctx, bg); err != nil {
		log.Error(err, "failed to process recurring breakglass")
		return ctrl.Result{}, err
	}

	// Check if it's time to activate
	if h.handler.RecurringManager.ShouldActivate(ctx, bg) {
		log.Info("recurring breakglass activation due, granting access")
		return h.handler.GrantAndActivate(ctx, bg)
	}

	// Calculate requeue time based on next activation
	if bg.Status.NextActivationAt != nil {
		untilNext := h.handler.Clock.Until(bg.Status.NextActivationAt.Time)

		// Cap the requeue time to a reasonable maximum (1 hour)
		maxInterval := time.Hour
		if untilNext > maxInterval {
			untilNext = maxInterval
		}

		// Ensure minimum requeue time (30 seconds)
		minInterval := 30 * time.Second
		if untilNext < minInterval {
			untilNext = minInterval
		}

		return ctrl.Result{RequeueAfter: untilNext}, nil
	}

	// Fallback requeue
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}
