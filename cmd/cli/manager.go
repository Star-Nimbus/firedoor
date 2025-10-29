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

package cli

import (
	"context"
	"crypto/tls"
	"fmt"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spf13/cobra"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/clock"
	"github.com/cloud-nimbus/firedoor/internal/config"
	"github.com/cloud-nimbus/firedoor/internal/constants"
	"github.com/cloud-nimbus/firedoor/internal/controller/breakglass"
	"github.com/cloud-nimbus/firedoor/internal/errors"
	"github.com/cloud-nimbus/firedoor/internal/operator/recurring"
	"github.com/cloud-nimbus/firedoor/internal/telemetry"
	//+kubebuilder:scaffold:imports
)

const (
	// Manager command descriptions
	managerCmdShort = "Start the Firedoor controller manager"
	managerCmdLong  = `Start the Firedoor controller manager which watches for Breakglass resources
and manages their lifecycle in the Kubernetes cluster.`
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(accessv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

// newManagerCmd creates the manager command
func newManagerCmd() *cobra.Command {
	var logLevel string

	cmd := &cobra.Command{
		Use:   "manager",
		Short: managerCmdShort,
		Long:  managerCmdLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runManager(cmd.Context(), cfg, logLevel)
		},
	}
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	return cmd
}

func runManager(ctx context.Context, cfg *config.Config, logLevel string) error {
	// Setup telemetry (logging, tracing, and metrics)
	effectiveLogLevel := logLevel
	if cfg.OTel.LogLevel != "" {
		effectiveLogLevel = cfg.OTel.LogLevel
	}
	shutdown, err := telemetry.Setup(
		ctx, cfg, "firedoor", "v1.0.0", effectiveLogLevel,
	)
	if err != nil {
		return fmt.Errorf("failed to setup telemetry: %w", err)
	}
	defer shutdown()

	metricsAddr := cfg.Metrics.BindAddress
	probeAddr := cfg.Health.ProbeBindAddress
	enableLeaderElection := cfg.Manager.LeaderElect
	secureMetrics := cfg.Metrics.Secure
	enableHTTP2 := cfg.HTTP.EnableHTTP2

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			setupLog.Info(errors.ErrDisableHTTP2)
			c.NextProtos = []string{"http/1.1"}
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{TLSOpts: tlsOpts})
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}
	if secureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       constants.LeaderElectionID,
	})
	if err != nil {
		setupLog.Error(err, errors.ErrStartManager)
		return err
	}

	// Register the Breakglass controller
	if err := breakglass.NewBreakglassReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		breakglass.WithRecurringManager(recurring.New(clock.SimpleClock{})),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create Breakglass controller")
		return err
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, errors.ErrSetupHealthCheck)
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, errors.ErrSetupReadyCheck)
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, errors.ErrRunManager)
		return err
	}

	return nil
}
