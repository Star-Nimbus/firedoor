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
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("Config", func() {
	Describe("Load", func() {
		Context("with default values", func() {
			It("should load configuration with correct defaults", func() {
				cfg, err := Load()
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg).NotTo(BeNil())

				// OpenTelemetry defaults
				Expect(cfg.OTel.Enabled).To(BeFalse())
				Expect(cfg.OTel.Exporter).To(Equal("otlp"))
				Expect(cfg.OTel.Endpoint).To(Equal("otel-collector-opentelemetry-collector.telemetry-system.svc.cluster.local:4317"))
				Expect(cfg.OTel.Service).To(Equal("firedoor-operator"))

				// Manager defaults
				Expect(cfg.Manager.LeaderElect).To(BeFalse())

				// Metrics defaults
				Expect(cfg.Metrics.BindAddress).To(Equal(":8080"))
				Expect(cfg.Metrics.Secure).To(BeFalse())

				// Health defaults
				Expect(cfg.Health.ProbeBindAddress).To(Equal(":8081"))

				// HTTP defaults
				Expect(cfg.HTTP.EnableHTTP2).To(BeFalse())
			})
		})

		Context("with environment variables", func() {
			BeforeEach(func() {
				_ = os.Setenv("FD_OTEL_ENABLED", "true")
				_ = os.Setenv("FD_OTEL_EXPORTER", "stdout")
				_ = os.Setenv("FD_OTEL_ENDPOINT", "http://custom:4318/v1/traces")
				_ = os.Setenv("FD_OTEL_SERVICE", "test-service")
				_ = os.Setenv("FD_MANAGER_LEADER_ELECT", "true")
				_ = os.Setenv("FD_METRICS_BIND_ADDRESS", ":9090")
				_ = os.Setenv("FD_METRICS_SECURE", "true")
				_ = os.Setenv("FD_HEALTH_PROBE_BIND_ADDRESS", ":9091")
				_ = os.Setenv("FD_HTTP_ENABLE_HTTP2", "true")
			})

			AfterEach(func() {
				// Clean up environment variables
				_ = os.Unsetenv("FD_OTEL_ENABLED")
				_ = os.Unsetenv("FD_OTEL_EXPORTER")
				_ = os.Unsetenv("FD_OTEL_ENDPOINT")
				_ = os.Unsetenv("FD_OTEL_SERVICE")
				_ = os.Unsetenv("FD_MANAGER_LEADER_ELECT")
				_ = os.Unsetenv("FD_METRICS_BIND_ADDRESS")
				_ = os.Unsetenv("FD_METRICS_SECURE")
				_ = os.Unsetenv("FD_HEALTH_PROBE_BIND_ADDRESS")
				_ = os.Unsetenv("FD_HTTP_ENABLE_HTTP2")
			})

			It("should load configuration from environment variables", func() {
				// Create a new viper instance for testing
				v := viper.New()

				// Set defaults
				setDefaults(v)

				// Read from environment variables
				v.SetEnvPrefix("FD")
				v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
				v.AutomaticEnv()

				var cfg Config
				err := v.Unmarshal(&cfg)
				Expect(err).NotTo(HaveOccurred())

				// OpenTelemetry values
				Expect(cfg.OTel.Enabled).To(BeTrue())
				Expect(cfg.OTel.Exporter).To(Equal("stdout"))
				Expect(cfg.OTel.Endpoint).To(Equal("http://custom:4318/v1/traces"))
				Expect(cfg.OTel.Service).To(Equal("test-service"))

				// Manager values
				Expect(cfg.Manager.LeaderElect).To(BeTrue())

				// Metrics values
				Expect(cfg.Metrics.BindAddress).To(Equal(":9090"))
				Expect(cfg.Metrics.Secure).To(BeTrue())

				// Health values
				Expect(cfg.Health.ProbeBindAddress).To(Equal(":9091"))

				// HTTP values
				Expect(cfg.HTTP.EnableHTTP2).To(BeTrue())
			})
		})

		Context("with LoadWithViper", func() {
			It("should work with a custom viper instance", func() {
				v := viper.New()
				v.Set("otel.enabled", true)
				v.Set("otel.service", "custom-service")
				v.Set("manager.leader_elect", true)
				v.Set("metrics.bind_address", ":8888")

				cfg, err := LoadWithViper(v)
				Expect(err).NotTo(HaveOccurred())

				Expect(cfg.OTel.Enabled).To(BeTrue())
				Expect(cfg.OTel.Service).To(Equal("custom-service"))
				Expect(cfg.Manager.LeaderElect).To(BeTrue())
				Expect(cfg.Metrics.BindAddress).To(Equal(":8888"))
			})
		})
	})

	Describe("OTelConfig", func() {
		Context("struct initialization", func() {
			It("should initialize correctly with values", func() {
				otelConfig := OTelConfig{
					Enabled:  true,
					Exporter: "otlp",
					Endpoint: "http://test:4318/v1/traces",
					Service:  "test-service",
				}

				Expect(otelConfig.Enabled).To(BeTrue())
				Expect(otelConfig.Exporter).To(Equal("otlp"))
				Expect(otelConfig.Endpoint).To(Equal("http://test:4318/v1/traces"))
				Expect(otelConfig.Service).To(Equal("test-service"))
			})

			It("should have correct mapstructure tags", func() {
				// Test that the struct can be properly unmarshaled
				v := viper.New()
				v.Set("otel.enabled", true)
				v.Set("otel.exporter", "stdout")
				v.Set("otel.endpoint", "http://example.com:4318/v1/traces")
				v.Set("otel.service", "example-service")
				v.Set("manager.leader_elect", true)
				v.Set("metrics.bind_address", ":8080")
				v.Set("metrics.secure", true)
				v.Set("health.probe_bind_address", ":8081")
				v.Set("http.enable_http2", true)

				var cfg Config
				err := v.Unmarshal(&cfg)
				Expect(err).NotTo(HaveOccurred())

				Expect(cfg.OTel.Enabled).To(BeTrue())
				Expect(cfg.OTel.Exporter).To(Equal("stdout"))
				Expect(cfg.OTel.Endpoint).To(Equal("http://example.com:4318/v1/traces"))
				Expect(cfg.OTel.Service).To(Equal("example-service"))
				Expect(cfg.Manager.LeaderElect).To(BeTrue())
				Expect(cfg.Metrics.BindAddress).To(Equal(":8080"))
				Expect(cfg.Metrics.Secure).To(BeTrue())
				Expect(cfg.Health.ProbeBindAddress).To(Equal(":8081"))
				Expect(cfg.HTTP.EnableHTTP2).To(BeTrue())
			})
		})
	})
})
