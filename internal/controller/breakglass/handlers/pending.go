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
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
)

// PendingCondition handles breakglass requests in the pending condition
type PendingCondition struct {
	handler *Handler
}

// NewPendingCondition creates a new PendingCondition
func NewPendingCondition(handler *Handler) *PendingCondition {
	return &PendingCondition{
		handler: handler,
	}
}

// Handle processes a pending breakglass request
func (h *PendingCondition) Handle(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// initialize Pending
	if len(bg.Status.Conditions) == 0 {
		if err := h.handler.updateStatus(
			ctx,
			bg,
			accessv1alpha1.ConditionPending,
			accessv1alpha1.ReasonNewResource,
			"Breakglass request is pending approval",
		); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Approval-required path
	if bg.Spec.Approval != nil && bg.Spec.Approval.Required {
		if bg.Status.ApprovedBy == "" {
			log.V(1).Info("waiting for approval")
			if err := h.handler.updateStatus(
				ctx,
				bg,
				accessv1alpha1.ConditionPending,
				accessv1alpha1.ReasonWaitingForApproval,
				"Breakglass request is pending approval",
			); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		log.V(1).Info("approval received, processing schedule")
		return h.handler.RecurringPendingCondition().Handle(ctx, bg)
	}

	// Auto-approve path
	log.V(1).Info("auto-approving breakglass")
	return h.handler.RecurringPendingCondition().Handle(ctx, bg)
}
