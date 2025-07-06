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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/cloud-nimbus/firedoor/cmd/cli"
)

func TestRoot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Root Suite")
}

var _ = Describe("Root Command", func() {
	var (
		rootCmd *cobra.Command
		output  *bytes.Buffer
	)

	BeforeEach(func() {
		rootCmd = cli.NewRootCmd()
		output = &bytes.Buffer{}
		rootCmd.SetOut(output)
		rootCmd.SetErr(output)
	})

	Describe("Command Structure", func() {
		It("should have correct command name and description", func() {
			Expect(rootCmd.Use).To(Equal("firedoor"))
			Expect(rootCmd.Short).To(ContainSubstring("Kubernetes operator"))
			Expect(rootCmd.Long).To(ContainSubstring("breakglass access"))
		})

		It("should have manager and version subcommands", func() {
			commands := rootCmd.Commands()
			Expect(commands).To(HaveLen(2))

			commandNames := make([]string, len(commands))
			for i, cmd := range commands {
				commandNames[i] = cmd.Use
			}
			Expect(commandNames).To(ContainElements("manager", "version"))
		})
	})

	Describe("Help Output", func() {
		It("should show help message", func() {
			rootCmd.SetArgs([]string{"--help"})
			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())
			Expect(output.String()).To(ContainSubstring("Firedoor is a Kubernetes operator"))
		})
	})

	Describe("Flag Handling", func() {
		It("should accept basic flags", func() {
			rootCmd.SetArgs([]string{"--config", "/test/config.yaml", "--help"})
			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle invalid flags", func() {
			rootCmd.SetArgs([]string{"--invalid-flag", "--help"})
			err := rootCmd.Execute()
			Expect(err).To(HaveOccurred())
		})
	})
})
