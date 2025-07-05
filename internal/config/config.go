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
	"strings"

	"github.com/spf13/viper"
)

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
	BindAddress string `mapstructure:"bind_address"`
	Secure      bool   `mapstructure:"secure"`
}

// HealthConfig holds health check configuration
type HealthConfig struct {
	ProbeBindAddress string `mapstructure:"probe_bind_address"`
}

// HTTPConfig holds HTTP server configuration
type HTTPConfig struct {
	EnableHTTP2 bool `mapstructure:"enable_http2"`
}

// Config holds all configuration settings for the application
type Config struct {
	OTel    OTelConfig    `mapstructure:"otel"`
	Manager ManagerConfig `mapstructure:"manager"`
	Metrics MetricsConfig `mapstructure:"metrics"`
	Health  HealthConfig  `mapstructure:"health"`
	HTTP    HTTPConfig    `mapstructure:"http"`
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

	return &config, nil
}

// setDefaults sets default values for all configuration options
func setDefaults(v *viper.Viper) {
	// OpenTelemetry defaults
	v.SetDefault("otel.enabled", false)
	v.SetDefault("otel.exporter", "otlp")
	v.SetDefault("otel.endpoint", "http://localhost:4318/v1/traces")
	v.SetDefault("otel.service", "firedoor-operator")

	// Manager defaults
	v.SetDefault("manager.leader_elect", false)

	// Metrics defaults
	v.SetDefault("metrics.bind_address", ":8080")
	v.SetDefault("metrics.secure", false)

	// Health defaults
	v.SetDefault("health.probe_bind_address", ":8081")

	// HTTP defaults
	v.SetDefault("http.enable_http2", false)
}
