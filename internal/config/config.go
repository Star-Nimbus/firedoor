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

// Config holds all configuration settings for the application
type Config struct {
	OTel OTelConfig `mapstructure:"otel"`
}

// Load reads configuration from various sources using viper
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("otel.enabled", false)
	v.SetDefault("otel.exporter", "otlp")
	v.SetDefault("otel.endpoint", "http://localhost:4318/v1/traces")
	v.SetDefault("otel.service", "firedoor-operator")

	// Read from environment variables
	v.SetEnvPrefix("FD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read from config file if it exists
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

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
