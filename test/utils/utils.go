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

package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
)

// warnError will use the global GINKGO_WRITER to log a warning message.
func warnError(err error) {
	_, _ = fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) ([]byte, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %s\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %s\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s failed with error: (%v) %s", command, err, string(output))
	}

	return output, nil
}

// LoadImageToKindCluster loads a local docker image to the kind cluster
func LoadImageToKindCluster(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	cmd := exec.Command("kind", kindOptions...)
	_, err := cmd.Output()
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breaks and prints only the non-empty lines
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.ReplaceAll(wd, "/test/e2e", "")
	return wd, nil
}

// SkaffoldRun deploys the application using Skaffold with the specified profile
func SkaffoldRun(profile string) error {
	// Check if skaffold is available
	if !isSkaffoldAvailable() {
		return fmt.Errorf("skaffold is not available in PATH. Please install skaffold to run e2e tests")
	}

	cmd := exec.Command("skaffold", "run", "--profile="+profile, "--tail=false")
	_, err := Run(cmd)
	return err
}

// SkaffoldDelete removes the application deployment using Skaffold
func SkaffoldDelete(profile string) error {
	// Check if skaffold is available
	if !isSkaffoldAvailable() {
		return fmt.Errorf("skaffold is not available in PATH")
	}

	cmd := exec.Command("skaffold", "delete", "--profile="+profile)
	_, err := Run(cmd)
	return err
}

// SkaffoldBuild builds the application using Skaffold without deploying
func SkaffoldBuild(profile string) error {
	// Check if skaffold is available
	if !isSkaffoldAvailable() {
		return fmt.Errorf("skaffold is not available in PATH")
	}

	cmd := exec.Command("skaffold", "build", "--profile="+profile)
	_, err := Run(cmd)
	return err
}

// SkaffoldDeploy deploys the application using Skaffold (assumes build is already done)
func SkaffoldDeploy(profile string) error {
	// Check if skaffold is available
	if !isSkaffoldAvailable() {
		return fmt.Errorf("skaffold is not available in PATH")
	}

	cmd := exec.Command("skaffold", "deploy", "--profile="+profile, "--tail=false")
	_, err := Run(cmd)
	return err
}

// isSkaffoldAvailable checks if skaffold is available in the PATH
func isSkaffoldAvailable() bool {
	cmd := exec.Command("skaffold", "version")
	return cmd.Run() == nil
}

// CleanupSkaffoldDeployment cleans up Skaffold deployment and any test resources
func CleanupSkaffoldDeployment(profile string) {
	By("cleaning up Skaffold deployment")
	if err := SkaffoldDelete(profile); err != nil {
		warnError(fmt.Errorf("failed to delete Skaffold deployment: %w", err))
	}

	// Clean up any test resources
	By("cleaning up test resources")
	cmd := exec.Command("kubectl", "delete", "breakglasses", "--all", "-n", "firedoor-system", "--ignore-not-found=true")
	if _, err := Run(cmd); err != nil {
		warnError(fmt.Errorf("failed to clean up test breakglass resources: %w", err))
	}
}

// WaitForCRD waits for the breakglass CRD to be available
func WaitForCRD() error {
	By("waiting for breakglass CRD to be available")
	cmd := exec.Command("kubectl", "get", "crd", "breakglasses.access.cloudnimbus.io", "--ignore-not-found=true")
	_, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to check CRD availability: %w", err)
	}

	// Wait a bit more to ensure the CRD is fully established
	time.Sleep(5 * time.Second)
	return nil
}

// WaitForCRDWithDiscovery waits until the given CRD's discovery doc is visible.
func WaitForCRDWithDiscovery(ctx context.Context, dc discovery.DiscoveryInterface, gvr schema.GroupVersionResource, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := dc.ServerResourcesForGroupVersion(gvr.GroupVersion().String())
		if discovery.IsGroupDiscoveryFailedError(err) || err == nil {
			// keep going until the specific resource appears
			resources, _ := dc.ServerResourcesForGroupVersion(gvr.GroupVersion().String())
			if resources != nil {
				for _, r := range resources.APIResources {
					if r.Name == gvr.Resource {
						fmt.Fprintf(GinkgoWriter, "Found CRD resource: %s\n", r.Name)
						return true, nil
					}
				}
			}
			fmt.Fprintf(GinkgoWriter, "Waiting for CRD resource: %s\n", gvr.Resource)
			return false, nil
		}
		return false, err // real error
	})
}
