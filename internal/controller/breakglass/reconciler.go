package breakglass

import (
	"context"
	"errors"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/config"
	"github.com/cloud-nimbus/firedoor/internal/controller"
	"github.com/cloud-nimbus/firedoor/internal/controller/breakglass/handlers"
	internalerrors "github.com/cloud-nimbus/firedoor/internal/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var tracer = otel.Tracer("firedoor/controller/breakglass")

const finalizer = "breakglass.firedoor.cloudnimbus.io/finalizer"

var defaultFactory = func(h *handlers.Handler) Controller { return handlers.NewPendingCondition(h) }

var handlerFactories = map[accessv1alpha1.BreakglassCondition]func(*handlers.Handler) Controller{
	accessv1alpha1.NoCondition:       defaultFactory,
	accessv1alpha1.ConditionPending:  func(h *handlers.Handler) Controller { return handlers.NewPendingCondition(h) },
	accessv1alpha1.ConditionApproved: func(h *handlers.Handler) Controller { return handlers.NewApprovedCondition(h) },
	accessv1alpha1.ConditionActive: func(h *handlers.Handler) Controller {
		return handlers.NewRecurringActiveCondition(h)
	},
	accessv1alpha1.ConditionDenied: func(h *handlers.Handler) Controller {
		return handlers.NewTerminalCondition(h, accessv1alpha1.ConditionDenied)
	},
	accessv1alpha1.ConditionExpired: func(h *handlers.Handler) Controller {
		return handlers.NewTerminalCondition(h, accessv1alpha1.ConditionExpired)
	},
	accessv1alpha1.ConditionRevoked: func(h *handlers.Handler) Controller {
		return handlers.NewTerminalCondition(h, accessv1alpha1.ConditionRevoked)
	},
	accessv1alpha1.ConditionRecurringPending: func(h *handlers.Handler) Controller {
		return handlers.NewRecurringPendingCondition(h)
	},
	accessv1alpha1.ConditionRecurringActive: func(h *handlers.Handler) Controller {
		return handlers.NewRecurringActiveCondition(h)
	},
}

// Controller is the interface for all handler types
// (must match the signature of Handle)
type Controller interface {
	Handle(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error)
}

// BreakglassReconciler watches Breakglass resources.
type BreakglassReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Operator         controller.BreakglassOperator
	RecurringManager controller.RecurringManager
	Alerts           controller.AlertService
	Clock            controller.Clock
	Config           *config.Config
	Telemetry        controller.TelemetrySink
	baseHandler      *handlers.Handler
	recorder         record.EventRecorder
}

// +kubebuilder:rbac:groups=access.cloudnimbus.io,resources=breakglasses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=access.cloudnimbus.io,resources=breakglasses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,
//
//	resources=rolebindings;clusterrolebindings;roles;clusterroles,
//	verbs=get;create;delete
//
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
func (r *BreakglassReconciler) currentCondition(bg *accessv1alpha1.Breakglass) accessv1alpha1.BreakglassCondition {
	if len(bg.Status.Conditions) == 0 {
		return accessv1alpha1.NoCondition
	}
	return accessv1alpha1.BreakglassCondition(bg.Status.Conditions[len(bg.Status.Conditions)-1].Type)
}

func (r *BreakglassReconciler) cleanupOnDelete(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	// 1) Revoke any external grants
	if err := r.Operator.RevokeAccess(ctx, bg); err != nil {
		// Check if this is a retryable error
		var rbacErr *internalerrors.RBACError
		if errors.As(err, &rbacErr) && rbacErr.IsRetryable() {
			return fmt.Errorf("revoke external access (retryable): %w", err)
		}
		return fmt.Errorf("revoke external access: %w", err)
	}

	// 2) Delete all RBAC resources via the operator
	if err := r.Operator.CleanupResources(ctx, bg); err != nil {
		// Check if this is a retryable error
		var rbacErr *internalerrors.RBACError
		if errors.As(err, &rbacErr) && rbacErr.IsRetryable() {
			return fmt.Errorf("cleanup RBAC resources (retryable): %w", err)
		}
		return fmt.Errorf("cleanup RBAC resources: %w", err)
	}

	// Emit event for successful cleanup
	if r.recorder != nil {
		r.recorder.Eventf(bg, "Normal", "CleanupCompleted",
			"Successfully cleaned up all RBAC resources for breakglass deletion")
	}

	return nil
}

func (r *BreakglassReconciler) reconcileFinalizers(
	ctx context.Context,
	bg *accessv1alpha1.Breakglass,
) (ctrl.Result, error) {
	if bg.DeletionTimestamp.IsZero() {
		if controllerutil.AddFinalizer(bg, finalizer) {
			return ctrl.Result{Requeue: true}, r.Client.Update(ctx, bg)
		}
		return ctrl.Result{}, nil
	}

	// CR is being deleted: do our cleanup
	if err := r.cleanupOnDelete(ctx, bg); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "cleanup on delete failed, will retry")
		return ctrl.Result{RequeueAfter: r.baseHandler.Backoff}, nil
	}

	// all cleaned up → remove our finalizer and let the CR go away
	controllerutil.RemoveFinalizer(bg, finalizer)
	return ctrl.Result{}, r.Client.Update(ctx, bg)
}

func (r *BreakglassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, span := tracer.Start(ctx, "Reconcile")
	defer span.End()

	bg, err := r.fetchAndInit(ctx, req)
	if bg == nil || err != nil {
		return ctrl.Result{}, err
	}

	bg, err = getFresh(ctx, r.Client, req.NamespacedName, &accessv1alpha1.Breakglass{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			ctrl.LoggerFrom(ctx).V(1).Info("breakglass deleted during reconciliation")
			return ctrl.Result{}, nil
		}
		ctrl.LoggerFrom(ctx).Error(err, "unable to fetch latest Breakglass for reconciliation")
		return ctrl.Result{}, err
	}
	ctrl.LoggerFrom(ctx).V(1).Info("fetched latest breakglass resource for reconciliation")

	// Default start and end if not set
	now := metav1.Now()

	if bg.Spec.Schedule == (accessv1alpha1.ScheduleSpec{}) {
		err := fmt.Errorf("schedule spec is required")
		ctrl.LoggerFrom(ctx).Error(err, "invalid schedule")
		r.recorder.Eventf(bg, "Warning", "InvalidSchedule", "schedule spec is required")
		return ctrl.Result{}, err
	}

	if bg.Spec.Schedule.Start.IsZero() {
		log.Printf("Start time not set; defaulting to now")
		ctrl.LoggerFrom(ctx).V(1).Info("start time not set; defaulting to now")
		if bg.Spec.Schedule.Cron == "" {
			err := fmt.Errorf("start time must be set for one-time schedules")
			ctrl.LoggerFrom(ctx).Error(err, "invalid schedule")
			r.recorder.Eventf(bg, "Warning", "InvalidSchedule", "start time must be set for one-time schedules")
			return ctrl.Result{}, err
		}
		bg.Spec.Schedule.Start = now
	}
	if bg.Spec.Schedule.Cron != "" && bg.Spec.Schedule.Duration.Duration <= 0 {
		return ctrl.Result{}, fmt.Errorf("Duration must be greater than 0 for recurring schedules")
	}

	if res, err := r.reconcileFinalizers(ctx, bg); res.Requeue || err != nil {
		return res, err
	}

	cond := r.currentCondition(bg)
	factory, found := handlerFactories[cond]
	if !found {
		ctrl.LoggerFrom(ctx).
			V(1).
			Info("unrecognized condition; defaulting to pending", "condition", cond)
		factory = defaultFactory
	}
	ctx, condSpan := tracer.Start(ctx, string(cond))
	defer condSpan.End()
	handler := factory(r.baseHandler)
	if handler == nil {
		err := fmt.Errorf("no handler for condition %q", cond)
		ctrl.LoggerFrom(ctx).Error(err, "invalid handler")
		return ctrl.Result{}, err
	}

	return handler.Handle(ctx, bg)
}

// fetchAndInit fetches the Breakglass resource and logs initial info
func (r *BreakglassReconciler) fetchAndInit(
	ctx context.Context,
	req ctrl.Request,
) (*accessv1alpha1.Breakglass, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("breakglass", req.NamespacedName)
	ctx = ctrl.LoggerInto(ctx, log)

	log.V(1).Info("starting reconciliation")

	bg, err := getFresh(ctx, r.Client, req.NamespacedName, &accessv1alpha1.Breakglass{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("breakglass resource not found; assuming it was deleted")
			return nil, nil
		}
		log.Error(err, "unable to fetch Breakglass")
		return nil, err
	}

	conditionType := "none"
	reason := "none"
	if len(bg.Status.Conditions) > 0 {
		lastCond := bg.Status.Conditions[len(bg.Status.Conditions)-1]
		conditionType = lastCond.Type
		reason = lastCond.Reason
	}
	log.V(1).Info("fetched breakglass resource",
		"name", bg.GetName(),
		"namespace", bg.GetNamespace(),
		"condition", conditionType,
		"reason", reason,
		"expiresAt", bg.Status.ExpiresAt,
	)
	return bg, nil
}

func getFresh[T client.Object](ctx context.Context, c client.Client, key client.ObjectKey, obj T) (T, error) {
	// Use a background context with the same values for clarity
	if err := c.Get(ctx, key, obj); err != nil {
		// Return typed nil if not found
		var zero T
		return zero, err
	}
	return obj, nil
}
