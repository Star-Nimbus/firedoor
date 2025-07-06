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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cloud-nimbus/firedoor/internal/config"
	"github.com/cloud-nimbus/firedoor/internal/errors"
)

const (
	// Root command descriptions
	rootCmdShort = "Firedoor is a Kubernetes operator for managing breakglass access"
	rootCmdLong  = `Firedoor is a Kubernetes operator that provides secure breakglass access
to Kubernetes clusters. It allows temporary elevated access for emergency
situations while maintaining audit trails and compliance.`

	// Flag descriptions
	configFlagDesc             = "config file (default is $HOME/.firedoor.yaml)"
	metricsBindAddressDesc     = "The address the metric endpoint binds to"
	healthProbeBindAddressDesc = "The address the probe endpoint binds to"
	leaderElectDesc            = "Enable leader election for controller manager"
	metricsSecureDesc          = "If set the metrics endpoint is served securely"
	enableHTTP2Desc            = "If set, HTTP/2 will be enabled for the metrics and webhook servers"
	otelEnabledDesc            = "Enable OpenTelemetry tracing"
	otelExporterDesc           = "OpenTelemetry exporter type (otlp, stdout)"
	otelEndpointDesc           = "OpenTelemetry OTLP endpoint"
	otelServiceDesc            = "OpenTelemetry service name"
)

var (
	cfgFile string
	cfg     *config.Config
)

// NewRootCmd creates and configures the root command
func NewRootCmd() *cobra.Command {
	// rootCmd represents the base command when called without any subcommands
	rootCmd := &cobra.Command{
		Use:   "firedoor",
		Short: rootCmdShort,
		Long:  rootCmdLong,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			var err error
			cfg, err = config.LoadWithViper(viper.GetViper())
			if err != nil {
				return fmt.Errorf("%s: %w", errors.ErrLoadConfig, err)
			}
			return nil
		},
	}

	// Initialize configuration
	cobra.OnInitialize(initConfig)

	// Add persistent flags
	addPersistentFlags(rootCmd)

	// Add subcommands
	rootCmd.AddCommand(newManagerCmd())
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd
}

// addPersistentFlags adds persistent flags to the root command
func addPersistentFlags(cmd *cobra.Command) {
	// Global flags
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", configFlagDesc)

	// Manager flags
	cmd.PersistentFlags().String("metrics-bind-address", ":8080", metricsBindAddressDesc)
	cmd.PersistentFlags().String("health-probe-bind-address", ":8081", healthProbeBindAddressDesc)
	cmd.PersistentFlags().Bool("leader-elect", false, leaderElectDesc)
	cmd.PersistentFlags().Bool("metrics-secure", false, metricsSecureDesc)
	cmd.PersistentFlags().Bool("enable-http2", false, enableHTTP2Desc)

	// OpenTelemetry flags
	cmd.PersistentFlags().Bool("otel-enabled", false, otelEnabledDesc)
	cmd.PersistentFlags().String("otel-exporter", "otlp", otelExporterDesc)
	cmd.PersistentFlags().String("otel-endpoint", "http://localhost:4318/v1/traces", otelEndpointDesc)
	cmd.PersistentFlags().String("otel-service", "firedoor-operator", otelServiceDesc)

	// Bind flags to viper
	if err := viper.BindPFlag("metrics.bind_address", cmd.PersistentFlags().Lookup("metrics-bind-address")); err != nil {
		// This should not happen in normal operation, but we handle it gracefully
		cobra.CheckErr(err)
	}
	healthFlag := cmd.PersistentFlags().Lookup("health-probe-bind-address")
	if err := viper.BindPFlag("health.probe_bind_address", healthFlag); err != nil {
		cobra.CheckErr(err)
	}
	if err := viper.BindPFlag("manager.leader_elect", cmd.PersistentFlags().Lookup("leader-elect")); err != nil {
		cobra.CheckErr(err)
	}
	if err := viper.BindPFlag("metrics.secure", cmd.PersistentFlags().Lookup("metrics-secure")); err != nil {
		cobra.CheckErr(err)
	}
	if err := viper.BindPFlag("http.enable_http2", cmd.PersistentFlags().Lookup("enable-http2")); err != nil {
		cobra.CheckErr(err)
	}

	if err := viper.BindPFlag("otel.enabled", cmd.PersistentFlags().Lookup("otel-enabled")); err != nil {
		cobra.CheckErr(err)
	}
	if err := viper.BindPFlag("otel.exporter", cmd.PersistentFlags().Lookup("otel-exporter")); err != nil {
		cobra.CheckErr(err)
	}
	if err := viper.BindPFlag("otel.endpoint", cmd.PersistentFlags().Lookup("otel-endpoint")); err != nil {
		cobra.CheckErr(err)
	}
	if err := viper.BindPFlag("otel.service", cmd.PersistentFlags().Lookup("otel-service")); err != nil {
		cobra.CheckErr(err)
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// GetConfig returns the loaded configuration
func GetConfig() *config.Config {
	return cfg
}

// initConfig reads in config file and ENV variables.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".firedoor" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc/firedoor")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".firedoor")
		viper.SetConfigName("config")
	}

	// Read in environment variables
	viper.SetEnvPrefix("FD")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
