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

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloud-nimbus/firedoor/test/utils"
)

const skaffoldNamespace = "firedoor-system"

var _ = Describe("Skaffold Integration", Ordered, func() {
	var clientset *kubernetes.Clientset
	var ctx context.Context

	BeforeAll(func() {
		ctx = context.Background()

		By("creating kubernetes client")
		config, err := clientcmd.BuildConfigFromFlags("", "")
		Expect(err).NotTo(HaveOccurred())

		clientset, err = kubernetes.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred())

		By("creating manager namespace")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: skaffoldNamespace,
			},
		}
		_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		if err != nil {
			// Namespace might already exist, ignore error
			fmt.Printf("Namespace creation failed (might already exist): %v\n", err)
		}
	})

	Context("Skaffold Profiles", func() {
		It("should deploy with dev profile", func() {
			By("installing CRDs")
			cmd := exec.Command("make", "install")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("deploying with dev profile")
			Expect(utils.SkaffoldRun("dev")).To(Succeed())

			By("waiting for deployment to be ready")
			Expect(utils.WaitForSkaffoldDeployment("dev", 2*time.Minute)).To(Succeed())

			By("verifying controller is running")
			Eventually(func() error {
				listOptions := metav1.ListOptions{
					LabelSelector: "control-plane=controller-manager",
				}
				podList, err := clientset.CoreV1().Pods(skaffoldNamespace).List(ctx, listOptions)
				if err != nil {
					return fmt.Errorf("failed to list pods: %w", err)
				}

				if len(podList.Items) == 0 {
					return fmt.Errorf("no controller pods found")
				}

				for _, pod := range podList.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return fmt.Errorf("pod %s is not running: %s", pod.Name, pod.Status.Phase)
					}
				}
				return nil
			}, time.Minute*2, time.Second).Should(Succeed())

			By("cleaning up dev deployment")
			utils.CleanupSkaffoldDeployment("dev")
		})

		It("should deploy with telemetry profile", func() {
			By("installing CRDs")
			cmd := exec.Command("make", "install")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("deploying with telemetry profile")
			Expect(utils.SkaffoldRun("telemetry")).To(Succeed())

			By("waiting for deployment to be ready")
			Expect(utils.WaitForSkaffoldDeployment("telemetry", 3*time.Minute)).To(Succeed())

			By("verifying telemetry components are running")
			Eventually(func() error {
				// Check for OpenTelemetry collector
				cmd := exec.Command("kubectl", "get", "pods", "-n", "telemetry-system", "-l", "app=opentelemetry-collector")
				_, err := utils.Run(cmd)
				return err
			}, time.Minute*2, time.Second).Should(Succeed())

			By("cleaning up telemetry deployment")
			utils.CleanupSkaffoldDeployment("telemetry")
		})

		It("should deploy with metrics profile", func() {
			By("installing CRDs")
			cmd := exec.Command("make", "install")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("deploying with metrics profile")
			Expect(utils.SkaffoldRun("metrics")).To(Succeed())

			By("waiting for deployment to be ready")
			Expect(utils.WaitForSkaffoldDeployment("metrics", 3*time.Minute)).To(Succeed())

			By("verifying metrics components are running")
			Eventually(func() error {
				// Check for Prometheus operator
				cmd := exec.Command("kubectl", "get", "pods", "-n", "monitoring-system", "-l", "app=prometheus-operator")
				_, err := utils.Run(cmd)
				return err
			}, time.Minute*2, time.Second).Should(Succeed())

			By("cleaning up metrics deployment")
			utils.CleanupSkaffoldDeployment("metrics")
		})

		It("should handle build and deploy separately", func() {
			By("installing CRDs")
			cmd := exec.Command("make", "install")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("building with dev profile")
			Expect(utils.SkaffoldBuild("dev")).To(Succeed())

			By("deploying pre-built artifacts")
			Expect(utils.SkaffoldDeploy("dev")).To(Succeed())

			By("waiting for deployment to be ready")
			Expect(utils.WaitForSkaffoldDeployment("dev", 2*time.Minute)).To(Succeed())

			By("cleaning up deployment")
			utils.CleanupSkaffoldDeployment("dev")
		})
	})

	Context("Breakglass CRD Operations", func() {
		BeforeEach(func() {
			By("installing CRDs")
			cmd := exec.Command("make", "install")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("deploying operator")
			Expect(utils.SkaffoldRun("dev")).To(Succeed())
			Expect(utils.WaitForSkaffoldDeployment("dev", 2*time.Minute)).To(Succeed())
		})

		AfterEach(func() {
			By("cleaning up deployment")
			utils.CleanupSkaffoldDeployment("dev")
		})

		It("should create and manage breakglass resources", func() {
			By("creating a breakglass resource with user")
			breakglassUserYAML := `
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: test-user-breakglass
  namespace: firedoor-system
spec:
  user: "test-user"
  namespace: "default"
  role: "admin"
  durationMinutes: 60
  approved: true
  reason: "E2E test - user access"
`
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(breakglassUserYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying user breakglass resource is created")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "breakglass", "test-user-breakglass", "-n", "firedoor-system")
				_, err := utils.Run(cmd)
				return err
			}, time.Minute, time.Second).Should(Succeed())

			By("creating a breakglass resource with group")
			breakglassGroupYAML := `
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: test-group-breakglass
  namespace: firedoor-system
spec:
  group: "test-group"
  namespace: "default"
  role: "admin"
  durationMinutes: 60
  approved: true
  reason: "E2E test - group access"
`
			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(breakglassGroupYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying group breakglass resource is created")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "breakglass", "test-group-breakglass", "-n", "firedoor-system")
				_, err := utils.Run(cmd)
				return err
			}, time.Minute, time.Second).Should(Succeed())

			By("listing all breakglass resources")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "breakglass", "-n", "firedoor-system")
				output, err := utils.Run(cmd)
				if err != nil {
					return err
				}
				// Should contain both resources
				if !strings.Contains(string(output), "test-user-breakglass") || !strings.Contains(string(output), "test-group-breakglass") {
					return fmt.Errorf("expected both breakglass resources to be listed")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())

			By("cleaning up test resources")
			cmd = exec.Command("kubectl", "delete", "breakglass", "test-user-breakglass", "-n", "firedoor-system")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command("kubectl", "delete", "breakglass", "test-group-breakglass", "-n", "firedoor-system")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
