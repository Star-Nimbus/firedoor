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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloud-nimbus/firedoor/test/utils"
)

const namespace = "firedoor-system"

var _ = Describe("controller", Ordered, func() {
	var clientset *kubernetes.Clientset
	var ctx context.Context

	BeforeAll(func() {
		ctx = context.Background()

		By("creating kubernetes client")
		config, err := clientcmd.BuildConfigFromFlags("", "")
		Expect(err).NotTo(HaveOccurred())

		clientset, err = kubernetes.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred())

		By("installing prometheus operator")
		Expect(utils.InstallPrometheusOperator()).To(Succeed())

		By("installing the cert-manager")
		Expect(utils.InstallCertManager()).To(Succeed())

		By("creating manager namespace")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		if err != nil {
			// Namespace might already exist, ignore error
			fmt.Printf("Namespace creation failed (might already exist): %v\n", err)
		}
	})

	AfterAll(func() {
		By("uninstalling the Prometheus manager bundle")
		utils.UninstallPrometheusOperator()

		By("uninstalling the cert-manager bundle")
		utils.UninstallCertManager()
	})

	Context("Operator", func() {
		It("should run successfully", func() {
			var err error

			// projectimage stores the name of the image used in the example
			const projectimage = "star-nimbus.io/firedoor:v0.0.1"

			By("building the manager(Operator) image")
			cmd := exec.Command("make", "docker-build", "IMG="+projectimage)
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("loading the the manager(Operator) image on Kind")
			err = utils.LoadImageToKindCluster(projectimage)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("installing CRDs")
			cmd = exec.Command("make", "install")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("deploying the controller-manager")
			cmd = exec.Command("make", "deploy", "IMG="+projectimage)
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func() error {
				// Get pods with label selector
				listOptions := metav1.ListOptions{
					LabelSelector: "control-plane=controller-manager",
				}

				podList, err := clientset.CoreV1().Pods(namespace).List(ctx, listOptions)
				if err != nil {
					return fmt.Errorf("failed to list pods: %w", err)
				}

				runningPods := []corev1.Pod{}
				for _, pod := range podList.Items {
					if pod.DeletionTimestamp == nil {
						runningPods = append(runningPods, pod)
					}
				}

				if len(runningPods) != 1 {
					return fmt.Errorf("expect 1 controller pods running, but got %d", len(runningPods))
				}

				controllerPod := runningPods[0]
				ExpectWithOffset(2, controllerPod.Name).Should(ContainSubstring("controller-manager"))

				// Validate pod status
				if controllerPod.Status.Phase != corev1.PodRunning {
					return fmt.Errorf("controller pod in %s status", controllerPod.Status.Phase)
				}

				return nil
			}
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())
		})
	})
})
