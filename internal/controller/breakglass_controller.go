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
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/config"
	"github.com/cloud-nimbus/firedoor/internal/telemetry"
)

var tracer = otel.Tracer("firedoor/internal/controller/breakglass")

// BreakglassReconciler reconciles a Breakglass object
type BreakglassReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Config   *config.Config
	operator BreakglassOperator
}

// +kubebuilder:rbac:groups=access.cloudnimbus.io,resources=breakglasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=access.cloudnimbus.io,resources=breakglasses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=access.cloudnimbus.io,resources=breakglasses/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;create;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BreakglassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, span := tracer.Start(ctx, "breakglass.reconcile")
	defer span.End()

	// Start metrics timer for reconciliation duration
	timer := telemetry.GetReconcileDurationTimer()
	defer timer.ObserveDuration()

	span.SetAttributes(
		attribute.String(telemetry.AttributeKeyRequestNamespace, req.Namespace),
		attribute.String(telemetry.AttributeKeyRequestName, req.Name),
	)

	bg := &accessv1alpha1.Breakglass{}
	if err := r.Client.Get(ctx, req.NamespacedName, bg); err != nil {
		span.RecordError(err)
		if client.IgnoreNotFound(err) == nil {
			telemetry.RecordReconciliationNotFound(req.Namespace)
		} else {
			telemetry.RecordReconciliationError(req.Namespace)
			telemetry.RecordError(
				telemetry.LabelComponentController,
				telemetry.LabelErrorTypeInternal,
				telemetry.LabelOperationReconcile,
			)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	span.SetAttributes(attribute.String(telemetry.AttributeKeyBreakglassName, bg.Name))

	// Record creation metric if this is a new breakglass
	if bg.Status.Phase == nil {
		telemetry.RecordBreakglassCreation(telemetry.LabelResultSuccess, telemetry.LabelComponentController, req.Namespace)
	}

	if isExpired(bg) {
		result, err := r.operator.RevokeAccess(ctx, bg)
		if err != nil {
			telemetry.RecordReconciliationExpired(req.Namespace, false)
			telemetry.RecordError(
				telemetry.LabelComponentController,
				telemetry.LabelErrorTypeInternal,
				telemetry.LabelOperationDelete,
			)
		} else {
			telemetry.RecordReconciliationExpired(req.Namespace, true)
		}
		return result, err
	}

	if bg.Spec.Approved && (bg.Status.Phase == nil || *bg.Status.Phase == accessv1alpha1.PhasePending) {
		result, err := r.operator.GrantAccess(ctx, bg)
		if err != nil {
			telemetry.RecordReconciliationActive(req.Namespace, false)
			telemetry.RecordError(
				telemetry.LabelComponentController,
				telemetry.LabelErrorTypeInternal,
				telemetry.LabelOperationCreate,
			)
		} else {
			telemetry.RecordReconciliationActive(req.Namespace, true)
		}
		return result, err
	}

	telemetry.RecordReconciliationNoAction(req.Namespace)
	return ctrl.Result{}, nil
}

// isExpired checks if the breakglass has expired
func isExpired(bg *accessv1alpha1.Breakglass) bool {
	return bg.Status.Phase != nil &&
		*bg.Status.Phase == accessv1alpha1.PhaseActive &&
		bg.Status.ExpiresAt != nil &&
		time.Now().After(bg.Status.ExpiresAt.Time)
}

func resolveSubject(bg *accessv1alpha1.Breakglass) (*rbacv1.Subject, error) {
	switch {
	case bg.Spec.Group != "":
		return &rbacv1.Subject{
			Kind: rbacv1.GroupKind,
			Name: bg.Spec.Group,
		}, nil
	case bg.Spec.User != "":
		return &rbacv1.Subject{
			Kind:     rbacv1.UserKind,
			Name:     bg.Spec.User,
			APIGroup: rbacv1.GroupName,
		}, nil
	default:
		return nil, fmt.Errorf("no user or group provided")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *BreakglassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.operator = NewBreakglassOperator(r.Client, r.Recorder)
	return ctrl.NewControllerManagedBy(mgr).
		For(&accessv1alpha1.Breakglass{}).
		Complete(r)
}
