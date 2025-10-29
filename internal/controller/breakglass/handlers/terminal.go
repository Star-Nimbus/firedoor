package handlers

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
)

type TerminalCondition struct {
	handler   *Handler
	condition accessv1alpha1.BreakglassCondition
}

func NewTerminalCondition(h *Handler, c accessv1alpha1.BreakglassCondition) *TerminalCondition {
	return &TerminalCondition{handler: h, condition: c}
}

func (t *TerminalCondition) Handle(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("condition", t.condition)
	log.V(1).Info("terminal state reached")
	return ctrl.Result{}, nil
}
