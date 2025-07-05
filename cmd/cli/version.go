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
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// Version information set by ldflags during build
	Version   = "dev"
	Commit    = "unknown"
	Date      = "unknown"
	GoVersion = runtime.Version()
	BuildBy   = "unknown"
)

// BuildInfo holds all build information
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"goVersion"`
	BuildBy   string `json:"buildBy"`
	Platform  string `json:"platform"`
}

// GetBuildInfo returns the build information
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: GoVersion,
		BuildBy:   BuildBy,
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// newVersionCmd creates the version command
func newVersionCmd() *cobra.Command {
	var (
		outputFormat string
		short        bool
	)

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long: `Print the version information for Firedoor including:
- Version number
- Git commit hash
- Build date
- Go version used to build
- Build platform
- Build source`,
		Run: func(cmd *cobra.Command, args []string) {
			out := cmd.OutOrStdout()
			buildInfo := GetBuildInfo()

			if short {
				_, _ = fmt.Fprintf(out, "%s\n", buildInfo.Version)
				return
			}

			switch outputFormat {
			case "json":
				encoder := json.NewEncoder(out)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(buildInfo); err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error encoding JSON: %v\n", err)
				}
			default:
				_, _ = fmt.Fprintf(out, "Firedoor %s\n", buildInfo.Version)
				_, _ = fmt.Fprintf(out, "Git commit: %s\n", buildInfo.Commit)
				_, _ = fmt.Fprintf(out, "Build date: %s\n", buildInfo.Date)
				_, _ = fmt.Fprintf(out, "Go version: %s\n", buildInfo.GoVersion)
				_, _ = fmt.Fprintf(out, "Platform: %s\n", buildInfo.Platform)
				_, _ = fmt.Fprintf(out, "Built by: %s\n", buildInfo.BuildBy)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")
	cmd.Flags().BoolVarP(&short, "short", "s", false, "Print only the version number")

	return cmd
}
