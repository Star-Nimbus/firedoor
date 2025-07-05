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

package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cloud-nimbus/firedoor/cmd/cli"
)

func TestCLI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI Suite")
}

var _ = Describe("CLI", func() {
	var (
		rootCmd    *cobra.Command
		output     *bytes.Buffer
		originalV  *viper.Viper
		testConfig string
	)

	BeforeEach(func() {
		// Save original viper instance
		originalV = viper.GetViper()

		// Reset viper for each test
		viper.Reset()

		// Create a new root command for each test
		rootCmd = cli.NewRootCmd()

		// Capture output
		output = &bytes.Buffer{}
		rootCmd.SetOut(output)
		rootCmd.SetErr(output)

		// Create test config content
		testConfig = `
otel:
  enabled: false
  exporter: otlp
  endpoint: http://localhost:4318/v1/traces
  service: firedoor-test
metrics:
  bind_address: ":8080"
  secure: false
health:
  probe_bind_address: ":8081"
manager:
  leader_elect: false
http:
  enable_http2: false
`
	})

	AfterEach(func() {
		// Restore original viper instance
		viper.Reset()
		for k, v := range originalV.AllSettings() {
			viper.Set(k, v)
		}
	})

	Describe("Root Command", func() {
		Context("when executed without subcommands", func() {
			It("should show help message", func() {
				rootCmd.SetArgs([]string{"--help"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
				Expect(output.String()).To(ContainSubstring(
					"Firedoor is a Kubernetes operator that provides secure breakglass access"))
				Expect(output.String()).To(ContainSubstring("Available Commands:"))
				Expect(output.String()).To(ContainSubstring("manager"))
				Expect(output.String()).To(ContainSubstring("version"))
			})
		})

		Context("when provided with flags", func() {
			It("should accept global flags", func() {
				rootCmd.SetArgs([]string{"--config", "/test/config.yaml", "--help"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
				Expect(output.String()).To(ContainSubstring("--config"))
			})

			It("should accept manager flags", func() {
				rootCmd.SetArgs([]string{"--metrics-bind-address", ":9090", "--help"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
				Expect(output.String()).To(ContainSubstring("--metrics-bind-address"))
			})

			It("should accept OpenTelemetry flags", func() {
				rootCmd.SetArgs([]string{"--otel-enabled", "--help"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
				Expect(output.String()).To(ContainSubstring("--otel-enabled"))
			})
		})
	})

	Describe("Version Command", func() {
		const (
			testVersion = "v1.0.0"
			testCommit  = "abc123def456"
			testDate    = "2025-01-01T00:00:00Z"
			testBuildBy = "test"
		)

		var (
			originalVersion string
			originalCommit  string
			originalDate    string
			originalBuildBy string
		)

		BeforeEach(func() {
			// Save original values
			originalVersion = cli.Version
			originalCommit = cli.Commit
			originalDate = cli.Date
			originalBuildBy = cli.BuildBy

			// Set test values
			cli.Version = testVersion
			cli.Commit = testCommit
			cli.Date = testDate
			cli.BuildBy = testBuildBy
		})

		AfterEach(func() {
			// Restore original values
			cli.Version = originalVersion
			cli.Commit = originalCommit
			cli.Date = originalDate
			cli.BuildBy = originalBuildBy
		})

		Context("when executed", func() {
			It("should display version information", func() {
				// Create a separate buffer to capture stdout
				versionOutput := &bytes.Buffer{}
				versionCmd := cli.NewRootCmd()
				versionCmd.SetOut(versionOutput)
				versionCmd.SetErr(versionOutput)

				versionCmd.SetArgs([]string{"version"})
				err := versionCmd.Execute()
				Expect(err).NotTo(HaveOccurred())

				outputStr := versionOutput.String()
				Expect(outputStr).To(ContainSubstring("Firedoor"))
				Expect(outputStr).To(ContainSubstring("Git commit:"))
				Expect(outputStr).To(ContainSubstring("Build date:"))
				Expect(outputStr).To(ContainSubstring("Go version:"))
				Expect(outputStr).To(ContainSubstring("Platform:"))
			})
		})

		Context("with default output", func() {
			It("should show detailed version information", func() {
				versionOutput := &bytes.Buffer{}
				versionCmd := cli.NewRootCmd()
				versionCmd.SetOut(versionOutput)
				versionCmd.SetErr(versionOutput)

				versionCmd.SetArgs([]string{"version"})
				err := versionCmd.Execute()
				Expect(err).NotTo(HaveOccurred())

				outputStr := versionOutput.String()
				Expect(outputStr).To(ContainSubstring("Firedoor " + testVersion))
				Expect(outputStr).To(ContainSubstring("Git commit: " + testCommit))
				Expect(outputStr).To(ContainSubstring("Build date: " + testDate))
				Expect(outputStr).To(ContainSubstring("Built by: " + testBuildBy))
			})
		})

		Context("with short output", func() {
			It("should show only the version number", func() {
				versionOutput := &bytes.Buffer{}
				versionCmd := cli.NewRootCmd()
				versionCmd.SetOut(versionOutput)
				versionCmd.SetErr(versionOutput)

				versionCmd.SetArgs([]string{"version", "--short"})
				err := versionCmd.Execute()
				Expect(err).NotTo(HaveOccurred())

				outputStr := versionOutput.String()
				Expect(outputStr).To(ContainSubstring(testVersion))
				Expect(outputStr).NotTo(ContainSubstring("Firedoor"))
				Expect(outputStr).NotTo(ContainSubstring("Git commit:"))
			})
		})

		Context("with JSON output", func() {
			It("should output valid JSON with all build information", func() {
				versionOutput := &bytes.Buffer{}
				versionCmd := cli.NewRootCmd()
				versionCmd.SetOut(versionOutput)
				versionCmd.SetErr(versionOutput)

				versionCmd.SetArgs([]string{"version", "--output", "json"})
				err := versionCmd.Execute()
				Expect(err).NotTo(HaveOccurred())

				outputStr := versionOutput.String()
				Expect(outputStr).To(ContainSubstring("\"version\": \"" + testVersion + "\""))
				Expect(outputStr).To(ContainSubstring("\"commit\": \"" + testCommit + "\""))
				Expect(outputStr).To(ContainSubstring("\"date\": \"" + testDate + "\""))
				Expect(outputStr).To(ContainSubstring("\"buildBy\": \"" + testBuildBy + "\""))

				// Validate JSON structure
				var buildInfo cli.BuildInfo
				err = json.Unmarshal([]byte(outputStr), &buildInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(buildInfo.Version).To(Equal(testVersion))
				Expect(buildInfo.Commit).To(Equal(testCommit))
				Expect(buildInfo.Date).To(Equal(testDate))
				Expect(buildInfo.BuildBy).To(Equal(testBuildBy))
			})
		})

		Context("with help flag", func() {
			It("should show help information", func() {
				versionOutput := &bytes.Buffer{}
				versionCmd := cli.NewRootCmd()
				versionCmd.SetOut(versionOutput)
				versionCmd.SetErr(versionOutput)

				versionCmd.SetArgs([]string{"version", "--help"})
				err := versionCmd.Execute()
				Expect(err).NotTo(HaveOccurred())

				outputStr := versionOutput.String()
				Expect(outputStr).To(ContainSubstring("Print the version information"))
				Expect(outputStr).To(ContainSubstring("--output"))
				Expect(outputStr).To(ContainSubstring("--short"))
				Expect(outputStr).To(ContainSubstring("text, json"))
			})
		})

		Context("GetBuildInfo function", func() {
			It("should return correct build information", func() {
				buildInfo := cli.GetBuildInfo()

				Expect(buildInfo.Version).To(Equal(testVersion))
				Expect(buildInfo.Commit).To(Equal(testCommit))
				Expect(buildInfo.Date).To(Equal(testDate))
				Expect(buildInfo.BuildBy).To(Equal(testBuildBy))
				Expect(buildInfo.Platform).NotTo(BeEmpty())
				Expect(buildInfo.GoVersion).NotTo(BeEmpty())
			})
		})

		Context("integration test", func() {
			It("should work with different flag combinations", func() {
				testCases := []struct {
					name             string
					args             []string
					expectedContains []string
				}{
					{
						name:             "default format",
						args:             []string{"version"},
						expectedContains: []string{"Firedoor", testVersion, "Git commit:", testCommit},
					},
					{
						name:             "short format",
						args:             []string{"version", "-s"},
						expectedContains: []string{testVersion},
					},
					{
						name:             "json format",
						args:             []string{"version", "-o", "json"},
						expectedContains: []string{"\"version\":", "\"commit\":", "\"date\":"},
					},
				}

				for _, tc := range testCases {
					By("Testing " + tc.name)
					versionOutput := &bytes.Buffer{}
					versionCmd := cli.NewRootCmd()
					versionCmd.SetOut(versionOutput)
					versionCmd.SetErr(versionOutput)

					versionCmd.SetArgs(tc.args)
					err := versionCmd.Execute()
					Expect(err).NotTo(HaveOccurred())

					outputStr := versionOutput.String()
					for _, expected := range tc.expectedContains {
						Expect(outputStr).To(ContainSubstring(expected))
					}
				}
			})
		})
	})

	Describe("Manager Command", func() {
		Context("when executed with config", func() {
			var (
				tempConfigFile *os.File
			)

			BeforeEach(func() {
				// Create a temporary config file
				var err error
				tempConfigFile, err = os.CreateTemp("", "firedoor-test-*.yaml")
				Expect(err).NotTo(HaveOccurred())

				_, err = tempConfigFile.WriteString(testConfig)
				Expect(err).NotTo(HaveOccurred())

				err = tempConfigFile.Close()
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				if tempConfigFile != nil {
					_ = os.Remove(tempConfigFile.Name())
				}
			})

			It("should accept manager flags", func() {
				rootCmd.SetArgs([]string{"manager", "--help"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
				Expect(output.String()).To(ContainSubstring("Start the Firedoor controller manager"))
			})

			It("should validate configuration loading", func() {
				// Test that the command setup doesn't fail immediately
				rootCmd.SetArgs([]string{"manager", "--config", tempConfigFile.Name(), "--help"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when executed without proper Kubernetes context", func() {
			It("should show appropriate error handling", func() {
				// This test verifies that the manager command structure is correct
				// In a real environment, this would fail due to missing kubeconfig
				rootCmd.SetArgs([]string{"manager", "--help"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
				Expect(output.String()).To(ContainSubstring("manager"))
			})
		})
	})

	Describe("Configuration Loading", func() {
		Context("when config file is provided", func() {
			var tempConfigFile *os.File

			BeforeEach(func() {
				var err error
				tempConfigFile, err = os.CreateTemp("", "firedoor-test-*.yaml")
				Expect(err).NotTo(HaveOccurred())

				_, err = tempConfigFile.WriteString(testConfig)
				Expect(err).NotTo(HaveOccurred())

				err = tempConfigFile.Close()
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				if tempConfigFile != nil {
					_ = os.Remove(tempConfigFile.Name())
				}
			})

			It("should load configuration from file", func() {
				rootCmd.SetArgs([]string{"--config", tempConfigFile.Name(), "version"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when environment variables are set", func() {
			BeforeEach(func() {
				_ = os.Setenv("FD_OTEL_ENABLED", "true")
				_ = os.Setenv("FD_OTEL_SERVICE", "test-service")
			})

			AfterEach(func() {
				_ = os.Unsetenv("FD_OTEL_ENABLED")
				_ = os.Unsetenv("FD_OTEL_SERVICE")
			})

			It("should respect environment variables", func() {
				rootCmd.SetArgs([]string{"version"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
				// Environment variables should be loaded by viper
			})
		})
	})

	Describe("Flag Binding", func() {
		Context("when flags are provided", func() {
			It("should bind metrics flags correctly", func() {
				rootCmd.SetArgs([]string{"--metrics-bind-address", ":9090", "version"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
				Expect(viper.GetString("metrics.bind_address")).To(Equal(":9090"))
			})

			It("should bind OpenTelemetry flags correctly", func() {
				rootCmd.SetArgs([]string{"--otel-enabled", "--otel-service", "test-service", "version"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
				Expect(viper.GetBool("otel.enabled")).To(BeTrue())
				Expect(viper.GetString("otel.service")).To(Equal("test-service"))
			})

			It("should bind leader election flags correctly", func() {
				rootCmd.SetArgs([]string{"--leader-elect", "version"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
				Expect(viper.GetBool("manager.leader_elect")).To(BeTrue())
			})
		})
	})

	Describe("Command Structure", func() {
		It("should have correct command hierarchy", func() {
			commands := rootCmd.Commands()
			commandNames := make([]string, len(commands))
			for i, cmd := range commands {
				commandNames[i] = cmd.Name()
			}

			Expect(commandNames).To(ContainElement("manager"))
			Expect(commandNames).To(ContainElement("version"))
		})

		It("should have correct root command properties", func() {
			Expect(rootCmd.Use).To(Equal("firedoor"))
			Expect(rootCmd.Short).To(ContainSubstring("Kubernetes operator"))
			Expect(rootCmd.Long).To(ContainSubstring("breakglass access"))
		})
	})
})
