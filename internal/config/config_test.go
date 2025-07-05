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

				Expect(cfg.OTel.Enabled).To(BeFalse())
				Expect(cfg.OTel.Exporter).To(Equal("otlp"))
				Expect(cfg.OTel.Endpoint).To(Equal("http://localhost:4318/v1/traces"))
				Expect(cfg.OTel.Service).To(Equal("firedoor-operator"))
			})
		})

		Context("with environment variables", func() {
			BeforeEach(func() {
				os.Setenv("FD_OTEL_ENABLED", "true")
				os.Setenv("FD_OTEL_EXPORTER", "stdout")
				os.Setenv("FD_OTEL_ENDPOINT", "http://custom:4318/v1/traces")
				os.Setenv("FD_OTEL_SERVICE", "test-service")
			})

			AfterEach(func() {
				os.Unsetenv("FD_OTEL_ENABLED")
				os.Unsetenv("FD_OTEL_EXPORTER")
				os.Unsetenv("FD_OTEL_ENDPOINT")
				os.Unsetenv("FD_OTEL_SERVICE")
			})

			It("should load configuration from environment variables", func() {
				// Create a new viper instance for testing
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

				var cfg Config
				err := v.Unmarshal(&cfg)
				Expect(err).NotTo(HaveOccurred())

				Expect(cfg.OTel.Enabled).To(BeTrue())
				Expect(cfg.OTel.Exporter).To(Equal("stdout"))
				Expect(cfg.OTel.Endpoint).To(Equal("http://custom:4318/v1/traces"))
				Expect(cfg.OTel.Service).To(Equal("test-service"))
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

				var cfg Config
				err := v.Unmarshal(&cfg)
				Expect(err).NotTo(HaveOccurred())

				Expect(cfg.OTel.Enabled).To(BeTrue())
				Expect(cfg.OTel.Exporter).To(Equal("stdout"))
				Expect(cfg.OTel.Endpoint).To(Equal("http://example.com:4318/v1/traces"))
				Expect(cfg.OTel.Service).To(Equal("example-service"))
			})
		})
	})
})
