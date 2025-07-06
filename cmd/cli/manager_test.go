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

func TestManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manager Suite")
}

var _ = Describe("Manager Command", func() {
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
		It("should have correct command properties", func() {
			managerCmd := rootCmd.Commands()[0]
			if managerCmd.Use != "manager" {
				managerCmd = rootCmd.Commands()[1]
			}

			Expect(managerCmd.Use).To(Equal("manager"))
			Expect(managerCmd.Short).To(ContainSubstring("Start the Firedoor controller manager"))
			Expect(managerCmd.Long).To(ContainSubstring("Breakglass resources"))
		})
	})

	Describe("Help Output", func() {
		It("should show manager help message", func() {
			rootCmd.SetArgs([]string{"manager", "--help"})
			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())
			Expect(output.String()).To(ContainSubstring("Start the Firedoor controller manager"))
		})
	})

	Describe("Configuration Integration", func() {
		It("should handle basic configuration", func() {
			rootCmd.SetArgs([]string{"manager", "--help"})
			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
