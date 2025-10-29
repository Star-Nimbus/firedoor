package breakglass

import (
	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/clock"
	"github.com/cloud-nimbus/firedoor/internal/config"
	"github.com/cloud-nimbus/firedoor/internal/controller/breakglass/handlers"
	"github.com/cloud-nimbus/firedoor/internal/operator/rbac"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewBreakglassReconciler creates a new BreakglassReconciler with the given options.
func NewBreakglassReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	opts ...Option,
) *BreakglassReconciler {
	r := &BreakglassReconciler{
		Client: client,
		Scheme: scheme,
	}
	// apply all options
	for _, o := range opts {
		o(r)
	}
	return r
}

func (r *BreakglassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Config == nil {
		r.Config = config.NewDefaultConfig()
	}
	if r.Clock == nil {
		r.Clock = clock.SimpleClock{}
	}
	if r.Operator == nil {
		r.Operator = rbac.New(mgr.GetClient())
	}
	if r.recorder == nil {
		r.recorder = mgr.GetEventRecorderFor("breakglass-controller")
	}
	r.baseHandler = handlers.NewHandler(
		r.Client, r.Operator, r.RecurringManager, r.Alerts, r.Clock, r.recorder, r.Config.Controller.Backoff,
	)
	return ctrl.NewControllerManagedBy(mgr).
		For(&accessv1alpha1.Breakglass{}).
		Complete(r)
}
