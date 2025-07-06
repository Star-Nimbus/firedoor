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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/cloud-nimbus/firedoor/cmd/cli"
)

func TestVersion(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Version Suite")
}

var _ = Describe("Version Command", func() {
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
			versionCmd := rootCmd.Commands()[1]
			if versionCmd.Use != "version" {
				versionCmd = rootCmd.Commands()[0]
			}

			Expect(versionCmd.Use).To(Equal("version"))
			Expect(versionCmd.Short).To(ContainSubstring("Print the version information"))
		})
	})

	Describe("Version Output", func() {
		It("should display version information in text format", func() {
			rootCmd.SetArgs([]string{"version"})
			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			outputStr := output.String()
			Expect(outputStr).To(ContainSubstring("Firedoor"))
			Expect(outputStr).To(ContainSubstring("Git commit:"))
			Expect(outputStr).To(ContainSubstring("Build date:"))
			Expect(outputStr).To(ContainSubstring("Go version:"))
			Expect(outputStr).To(ContainSubstring("Platform:"))
		})

		It("should display version in JSON format", func() {
			rootCmd.SetArgs([]string{"version", "--output", "json"})
			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			var buildInfo cli.BuildInfo
			err = json.Unmarshal(output.Bytes(), &buildInfo)
			Expect(err).NotTo(HaveOccurred())
			Expect(buildInfo.Version).NotTo(BeEmpty())
			Expect(buildInfo.Platform).NotTo(BeEmpty())
		})

		It("should display short version", func() {
			rootCmd.SetArgs([]string{"version", "--short"})
			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			outputStr := output.String()
			Expect(outputStr).NotTo(ContainSubstring("Git commit:"))
			Expect(outputStr).To(ContainSubstring("dev"))
		})
	})

	Describe("BuildInfo", func() {
		It("should return valid build information", func() {
			buildInfo := cli.GetBuildInfo()
			Expect(buildInfo.Version).NotTo(BeEmpty())
			Expect(buildInfo.Platform).NotTo(BeEmpty())
			Expect(buildInfo.GoVersion).NotTo(BeEmpty())
		})
	})
})
