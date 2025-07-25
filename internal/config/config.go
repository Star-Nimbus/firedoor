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

package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration settings for the application
type Config struct {
	OTel         OTelConfig         `mapstructure:"otel"`
	Manager      ManagerConfig      `mapstructure:"manager"`
	Metrics      MetricsConfig      `mapstructure:"metrics"`
	Health       HealthConfig       `mapstructure:"health"`
	HTTP         HTTPConfig         `mapstructure:"http"`
	Controller   ControllerConfig   `mapstructure:"controller"`
	Server       ServerConfig       `mapstructure:"server"`
	Alertmanager AlertmanagerConfig `mapstructure:"alertmanager"`
}

// OTelConfig holds OpenTelemetry configuration settings
type OTelConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Exporter string `mapstructure:"exporter"`
	Endpoint string `mapstructure:"endpoint"`
	Service  string `mapstructure:"service"`
}

// ManagerConfig holds manager-specific configuration
type ManagerConfig struct {
	LeaderElect bool `mapstructure:"leader_elect"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	BindAddress              string    `mapstructure:"bind_address"`
	Secure                   bool      `mapstructure:"secure"`
	ReconcileBuckets         []float64 `mapstructure:"reconcile_buckets"`
	ReconcileDurationBuckets []float64 `mapstructure:"reconcile_duration_buckets"`
	DurationBucketStart      float64   `mapstructure:"duration_bucket_start"`
	DurationBucketWidth      float64   `mapstructure:"duration_bucket_width"`
	DurationBucketCount      int       `mapstructure:"duration_bucket_count"`
}

// HealthConfig holds health check configuration
type HealthConfig struct {
	ProbeBindAddress string `mapstructure:"probe_bind_address"`
}

// HTTPConfig holds HTTP server configuration
type HTTPConfig struct {
	EnableHTTP2 bool `mapstructure:"enable_http2"`
}

// ControllerConfig holds controller-specific configuration
type ControllerConfig struct {
	ReconcileTimeout    time.Duration `mapstructure:"reconcile_timeout"`
	RetryDelay          time.Duration `mapstructure:"retry_delay"`
	PrivilegeEscalation bool          `mapstructure:"privilege_escalation"`
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	MetricsBindAddress     string `mapstructure:"metrics_bind_address"`
	HealthProbeBindAddress string `mapstructure:"health_probe_bind_address"`
	LeaderElect            bool   `mapstructure:"leader_elect"`
}

// AlertmanagerConfig holds Alertmanager configuration
type AlertmanagerConfig struct {
	// Enabled determines if Alertmanager integration is active
	Enabled bool `mapstructure:"enabled"`

	// URL is the Alertmanager API endpoint
	URL string `mapstructure:"url"`

	// Timeout for Alertmanager API requests
	Timeout time.Duration `mapstructure:"timeout"`

	// BasicAuth configuration
	BasicAuth BasicAuthConfig `mapstructure:"basic_auth"`

	// TLS configuration
	TLS TLSConfig `mapstructure:"tls"`

	// Alert configuration
	Alert AlertConfig `mapstructure:"alert"`
}

// BasicAuthConfig holds basic authentication configuration
type BasicAuthConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
	CAFile             string `mapstructure:"ca_file"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
}

// AlertConfig holds alert-specific configuration
type AlertConfig struct {
	// Labels to add to all alerts
	Labels map[string]string `mapstructure:"labels"`

	// Annotations to add to all alerts
	Annotations map[string]string `mapstructure:"annotations"`

	// Alert name template
	AlertName string `mapstructure:"alert_name"`

	// Severity level for breakglass alerts
	Severity string `mapstructure:"severity"`

	// Summary template for alerts
	Summary string `mapstructure:"summary"`

	// Description template for alerts
	Description string `mapstructure:"description"`
}

// Load reads configuration from various sources using viper
func Load() (*Config, error) {
	v := viper.New()
	return LoadWithViper(v)
}

// LoadWithViper reads configuration using the provided viper instance
func LoadWithViper(v *viper.Viper) (*Config, error) {
	// Set defaults
	setDefaults(v)

	// Read from environment variables
	v.SetEnvPrefix("FD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read from config file if it exists
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/firedoor") // ConfigMap mount path

	// Read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// setDefaults sets default values for all configuration options
func setDefaults(v *viper.Viper) {
	defaults := NewDefaults()

	// OpenTelemetry defaults
	v.SetDefault("otel.enabled", defaults.OTel.Enabled)
	v.SetDefault("otel.exporter", defaults.OTel.Exporter)
	v.SetDefault("otel.endpoint", defaults.OTel.Endpoint)
	v.SetDefault("otel.service", defaults.OTel.Service)

	// Manager defaults
	v.SetDefault("manager.leader_elect", defaults.Manager.LeaderElect)

	// Metrics defaults
	v.SetDefault("metrics.bind_address", defaults.Metrics.BindAddress)
	v.SetDefault("metrics.secure", defaults.Metrics.Secure)
	v.SetDefault("metrics.duration_bucket_start", defaults.Metrics.DurationBucketStart)
	v.SetDefault("metrics.duration_bucket_width", defaults.Metrics.DurationBucketWidth)
	v.SetDefault("metrics.duration_bucket_count", defaults.Metrics.DurationBucketCount)

	// Health defaults
	v.SetDefault("health.probe_bind_address", defaults.Health.ProbeBindAddress)

	// HTTP defaults
	v.SetDefault("http.enable_http2", defaults.HTTP.EnableHTTP2)

	// Controller defaults
	v.SetDefault("controller.reconcile_timeout", defaults.Controller.ReconcileTimeout)
	v.SetDefault("controller.retry_delay", defaults.Controller.RetryDelay)
	v.SetDefault("controller.privilege_escalation", defaults.Controller.PrivilegeEscalation)

	// Server defaults
	v.SetDefault("server.metrics_bind_address", defaults.Server.MetricsBindAddress)
	v.SetDefault("server.health_probe_bind_address", defaults.Server.HealthProbeBindAddress)
	v.SetDefault("server.leader_elect", defaults.Server.LeaderElect)

	// Alertmanager defaults
	v.SetDefault("alertmanager.enabled", defaults.Alertmanager.Enabled)
	v.SetDefault("alertmanager.url", defaults.Alertmanager.Endpoint)
	v.SetDefault("alertmanager.timeout", 30*time.Second)
	v.SetDefault("alertmanager.alert.alert_name", "BreakglassActive")
	v.SetDefault("alertmanager.alert.severity", "warning")
	v.SetDefault("alertmanager.alert.summary", "Breakglass access is active")
	v.SetDefault("alertmanager.alert.description", "A breakglass access request is currently active")
}

// Validate checks that all configuration values are valid
func (c *Config) Validate() error {
	if c.Metrics.DurationBucketStart <= 0 {
		return fmt.Errorf("metrics.duration_bucket_start must be greater than 0")
	}

	if c.Metrics.DurationBucketWidth <= 0 {
		return fmt.Errorf("metrics.duration_bucket_width must be greater than 0")
	}

	if c.Metrics.DurationBucketCount <= 0 {
		return fmt.Errorf("metrics.duration_bucket_count must be greater than 0")
	}

	return nil
}

// GetDurationBuckets returns the histogram buckets for duration metrics
func (c *Config) GetDurationBuckets() []float64 {
	buckets := make([]float64, c.Metrics.DurationBucketCount)
	for i := 0; i < c.Metrics.DurationBucketCount; i++ {
		buckets[i] = c.Metrics.DurationBucketStart + float64(i)*c.Metrics.DurationBucketWidth
	}
	return buckets
}

// NewDefaultConfig creates a config with default values
func NewDefaultConfig() *Config {
	defaults := NewDefaults()
	return &Config{
		OTel: OTelConfig{
			Enabled:  defaults.OTel.Enabled,
			Exporter: defaults.OTel.Exporter,
			Endpoint: defaults.OTel.Endpoint,
			Service:  defaults.OTel.Service,
		},
		Manager: ManagerConfig{
			LeaderElect: defaults.Manager.LeaderElect,
		},
		Metrics: MetricsConfig{
			BindAddress:         defaults.Metrics.BindAddress,
			Secure:              defaults.Metrics.Secure,
			DurationBucketStart: defaults.Metrics.DurationBucketStart,
			DurationBucketWidth: defaults.Metrics.DurationBucketWidth,
			DurationBucketCount: defaults.Metrics.DurationBucketCount,
		},
		Health: HealthConfig{
			ProbeBindAddress: defaults.Health.ProbeBindAddress,
		},
		HTTP: HTTPConfig{
			EnableHTTP2: defaults.HTTP.EnableHTTP2,
		},
		Controller: ControllerConfig{
			ReconcileTimeout:    defaults.Controller.ReconcileTimeout,
			RetryDelay:          defaults.Controller.RetryDelay,
			PrivilegeEscalation: defaults.Controller.PrivilegeEscalation,
		},
		Server: ServerConfig{
			MetricsBindAddress:     defaults.Server.MetricsBindAddress,
			HealthProbeBindAddress: defaults.Server.HealthProbeBindAddress,
			LeaderElect:            defaults.Server.LeaderElect,
		},
		Alertmanager: AlertmanagerConfig{
			Enabled:   defaults.Alertmanager.Enabled,
			URL:       defaults.Alertmanager.Endpoint,
			Timeout:   30 * time.Second,
			BasicAuth: BasicAuthConfig{},
			TLS: TLSConfig{
				InsecureSkipVerify: false,
			},
			Alert: AlertConfig{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				AlertName:   "BreakglassActive",
				Severity:    "warning",
				Summary:     "Breakglass access is active",
				Description: "A breakglass access request is currently active",
			},
		},
	}
}
