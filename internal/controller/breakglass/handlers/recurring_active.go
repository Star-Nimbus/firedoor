package handlers

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/controller/breakglass/usecases"
)

// RecurringActiveCondition handles breakglass requests in the recurring active condition
type RecurringActiveCondition struct {
	handler *Handler
}

// NewRecurringActiveCondition creates a new RecurringActiveCondition
func NewRecurringActiveCondition(handler *Handler) *RecurringActiveCondition {
	return &RecurringActiveCondition{handler: handler}
}

// Handle processes a recurring active breakglass request
func (h *RecurringActiveCondition) Handle(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	now := h.handler.Clock.Now()
	window, hasWindow := usecases.CurrentWindow(bg, now)
	// Check if the current access period has expired
	if hasWindow && h.handler.Clock.IsExpired(window.End) {
		log.Info("recurring breakglass access period expired, revoking access")
		return h.handler.RevokeAndExpire(ctx, bg)
	}

	// Process schedule logic to check for next activation
	if err := h.handler.RecurringManager.ProcessRecurring(ctx, bg); err != nil {
		log.Error(err, "failed to process recurring breakglass")
		return ctrl.Result{}, err
	}

	// If the condition changed to RecurringPending, requeue to handle it
	if len(bg.Status.Conditions) > 0 {
		lastCond := bg.Status.Conditions[len(bg.Status.Conditions)-1]
		if lastCond.Type == string(accessv1alpha1.ConditionRecurringPending) {
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// Calculate requeue time based on expiration or next activation
	if hasWindow {
		untilExpiry := h.handler.Clock.Until(window.End)

		// Cap the requeue time to a reasonable maximum (1 hour)
		maxInterval := time.Hour
		if untilExpiry > maxInterval {
			untilExpiry = maxInterval
		}

		// Ensure minimum requeue time (30 seconds)
		minInterval := 30 * time.Second
		if untilExpiry < minInterval {
			untilExpiry = minInterval
		}

		return ctrl.Result{RequeueAfter: untilExpiry}, nil
	}

	if bg.Status.NextActivationAt != nil {
		untilNext := h.handler.Clock.Until(bg.Status.NextActivationAt.Time)
		// ensure sane bounds similar to recurring pending
		maxInterval := time.Hour
		if untilNext > maxInterval {
			untilNext = maxInterval
		}
		minInterval := 30 * time.Second
		if untilNext < minInterval {
			untilNext = minInterval
		}
		return ctrl.Result{RequeueAfter: untilNext}, nil
	}

	// Fallback requeue
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}
