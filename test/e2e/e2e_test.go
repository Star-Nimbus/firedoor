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
	"log"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/conditions"
	"github.com/cloud-nimbus/firedoor/test/utils"
)

const namespace = "firedoor-system"

var _ = Describe("controller", Ordered, func() {
	var clientset *kubernetes.Clientset
	var k8sClient client.Client
	var ctx context.Context

	BeforeAll(func() {
		ctx = context.Background()

		// Setup kubernetes clientset
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		k8sConfig, err := kubeConfig.ClientConfig()
		if err != nil {
			log.Fatalf("failed to load kubeconfig: %v", err)
		}

		clientset, err = kubernetes.NewForConfig(k8sConfig)
		Expect(err).NotTo(HaveOccurred())

		// Setup controller-runtime client
		cfg, err := config.GetConfig()
		Expect(err).NotTo(HaveOccurred())

		// Create a new scheme and add the Breakglass type
		s := runtime.NewScheme()
		err = accessv1alpha1.AddToScheme(s)
		Expect(err).NotTo(HaveOccurred())

		k8sClient, err = client.New(cfg, client.Options{Scheme: s})
		Expect(err).NotTo(HaveOccurred())

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

	Context("Operator", func() {
		It("should run successfully", func() {
			By("deploying the controller-manager using Skaffold")
			Expect(utils.SkaffoldRun("dev")).To(Succeed())

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
			EventuallyWithOffset(1, verifyControllerUp, time.Minute*2, time.Second).Should(Succeed())

			By("cleaning up Skaffold deployment")
			utils.CleanupSkaffoldDeployment("dev")
		})

		It("should handle breakglass CRD operations with conditions", func() {
			By("deploying the controller-manager using Skaffold")
			Expect(utils.SkaffoldRun("dev")).To(Succeed())

			// Wait for controller to be ready and CRD to be available
			time.Sleep(10 * time.Second)
			Expect(utils.WaitForCRD()).To(Succeed())

			By("creating a valid breakglass resource with user")
			breakglass := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-breakglass-user",
					Namespace: namespace,
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: "User",
						Name: "test-user",
					}},
					AccessPolicy: &accessv1alpha1.AccessPolicy{
						Rules: []accessv1alpha1.AccessRule{{
							Actions:    []accessv1alpha1.Action{"get", "list"},
							Resources:  []string{"pods", "deployments"},
							Namespaces: []string{"default"},
						}},
					},
					ApprovalRequired: false,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "E2E test with user",
				},
			}

			Expect(k8sClient.Create(ctx, breakglass)).To(Succeed())

			By("verifying breakglass is created and conditions are set")
			Eventually(func() error {
				var bg accessv1alpha1.Breakglass
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-breakglass-user", Namespace: namespace}, &bg); err != nil {
					return err
				}

				// Check state is set
				if bg.Status.Phase == "" {
					return fmt.Errorf("state not set")
				}

				// Check conditions
				if len(bg.Status.Conditions) == 0 {
					return fmt.Errorf("no conditions set")
				}

				// Find approved condition
				var approvedCondition *metav1.Condition
				for i := range bg.Status.Conditions {
					if bg.Status.Conditions[i].Type == conditions.Approved.String() {
						approvedCondition = &bg.Status.Conditions[i]
						break
					}
				}

				if approvedCondition == nil {
					return fmt.Errorf("approved condition not found")
				}

				if approvedCondition.Status != metav1.ConditionTrue {
					return fmt.Errorf("approved condition not true, got: %s", approvedCondition.Status)
				}

				if approvedCondition.Reason != conditions.AccessGranted.String() {
					return fmt.Errorf("expected reason %s, got: %s", conditions.AccessGranted.String(), approvedCondition.Reason)
				}

				// Find active condition
				var activeCondition *metav1.Condition
				for i := range bg.Status.Conditions {
					if bg.Status.Conditions[i].Type == conditions.Active.String() {
						activeCondition = &bg.Status.Conditions[i]
						break
					}
				}

				if activeCondition == nil {
					return fmt.Errorf("active condition not found")
				}

				if activeCondition.Status != metav1.ConditionTrue {
					return fmt.Errorf("active condition not true, got: %s", activeCondition.Status)
				}

				if activeCondition.Reason != conditions.AccessActive.String() {
					return fmt.Errorf("expected reason %s, got: %s", conditions.AccessActive.String(), activeCondition.Reason)
				}

				return nil
			}, time.Minute*2, time.Second).Should(Succeed())

			By("cleaning up test breakglass resource")
			Expect(k8sClient.Delete(ctx, breakglass)).To(Succeed())

			By("cleaning up Skaffold deployment")
			utils.CleanupSkaffoldDeployment("dev")
		})

		It("should handle breakglass with group", func() {
			By("deploying the controller-manager using Skaffold")
			Expect(utils.SkaffoldRun("dev")).To(Succeed())

			// Wait for controller to be ready and CRD to be available
			time.Sleep(10 * time.Second)
			Expect(utils.WaitForCRD()).To(Succeed())

			By("creating a valid breakglass resource with group")
			breakglass := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-breakglass-group",
					Namespace: namespace,
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: "Group",
						Name: "test-group",
					}},
					AccessPolicy: &accessv1alpha1.AccessPolicy{
						Rules: []accessv1alpha1.AccessRule{{
							Actions:    []accessv1alpha1.Action{"get", "list"},
							Resources:  []string{"pods", "deployments"},
							Namespaces: []string{"default"},
						}},
					},
					ApprovalRequired: false,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "E2E test with group",
				},
			}

			Expect(k8sClient.Create(ctx, breakglass)).To(Succeed())

			By("verifying breakglass is created and conditions are set")
			Eventually(func() error {
				var bg accessv1alpha1.Breakglass
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-breakglass-group", Namespace: namespace}, &bg); err != nil {
					return err
				}

				// Check state is set
				if bg.Status.Phase == "" {
					return fmt.Errorf("state not set")
				}

				// Check conditions
				if len(bg.Status.Conditions) == 0 {
					return fmt.Errorf("no conditions set")
				}

				// Find approved condition
				var approvedCondition *metav1.Condition
				for i := range bg.Status.Conditions {
					if bg.Status.Conditions[i].Type == conditions.Approved.String() {
						approvedCondition = &bg.Status.Conditions[i]
						break
					}
				}

				if approvedCondition == nil {
					return fmt.Errorf("approved condition not found")
				}

				if approvedCondition.Status != metav1.ConditionTrue {
					return fmt.Errorf("approved condition not true, got: %s", approvedCondition.Status)
				}

				return nil
			}, time.Minute*2, time.Second).Should(Succeed())

			By("cleaning up test breakglass resource")
			Expect(k8sClient.Delete(ctx, breakglass)).To(Succeed())

			By("cleaning up Skaffold deployment")
			utils.CleanupSkaffoldDeployment("dev")
		})

		It("should handle invalid breakglass - missing user and group", func() {
			By("deploying the controller-manager using Skaffold")
			Expect(utils.SkaffoldRun("dev")).To(Succeed())

			// Wait for controller to be ready and CRD to be available
			time.Sleep(10 * time.Second)
			Expect(utils.WaitForCRD()).To(Succeed())

			By("creating an invalid breakglass resource without user or group")
			breakglass := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-breakglass-invalid",
					Namespace: namespace,
				},
				Spec: accessv1alpha1.BreakglassSpec{
					AccessPolicy: &accessv1alpha1.AccessPolicy{
						Rules: []accessv1alpha1.AccessRule{{
							Actions:    []accessv1alpha1.Action{"get", "list"},
							Resources:  []string{"pods", "deployments"},
							Namespaces: []string{"default"},
						}},
					},
					ApprovalRequired: false,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "E2E test invalid",
				},
			}

			Expect(k8sClient.Create(ctx, breakglass)).To(Succeed())

			By("verifying breakglass is created and denied conditions are set")
			Eventually(func() error {
				var bg accessv1alpha1.Breakglass
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-breakglass-invalid", Namespace: namespace}, &bg); err != nil {
					return err
				}

				// Check state is set to denied
				if bg.Status.Phase == "" {
					return fmt.Errorf("state not set")
				}

				if bg.Status.Phase != "Denied" {
					return fmt.Errorf("expected state Denied, got: %s", bg.Status.Phase)
				}

				// Check conditions
				if len(bg.Status.Conditions) == 0 {
					return fmt.Errorf("no conditions set")
				}

				// Find denied condition
				var deniedCondition *metav1.Condition
				for i := range bg.Status.Conditions {
					if bg.Status.Conditions[i].Type == conditions.Denied.String() {
						deniedCondition = &bg.Status.Conditions[i]
						break
					}
				}

				if deniedCondition == nil {
					return fmt.Errorf("denied condition not found")
				}

				if deniedCondition.Status != metav1.ConditionTrue {
					return fmt.Errorf("denied condition not true, got: %s", deniedCondition.Status)
				}

				if deniedCondition.Reason != conditions.InvalidRequest.String() {
					return fmt.Errorf("expected reason %s, got: %s", conditions.InvalidRequest.String(), deniedCondition.Reason)
				}

				// Find approved condition (should be false)
				var approvedCondition *metav1.Condition
				for i := range bg.Status.Conditions {
					if bg.Status.Conditions[i].Type == conditions.Approved.String() {
						approvedCondition = &bg.Status.Conditions[i]
						break
					}
				}

				if approvedCondition == nil {
					return fmt.Errorf("approved condition not found")
				}

				if approvedCondition.Status != metav1.ConditionFalse {
					return fmt.Errorf("approved condition not false, got: %s", approvedCondition.Status)
				}

				return nil
			}, time.Minute*2, time.Second).Should(Succeed())

			By("cleaning up test breakglass resource")
			Expect(k8sClient.Delete(ctx, breakglass)).To(Succeed())

			By("cleaning up Skaffold deployment")
			utils.CleanupSkaffoldDeployment("dev")
		})

		It("should handle breakglass expiration", func() {
			By("deploying the controller-manager using Skaffold")
			Expect(utils.SkaffoldRun("dev")).To(Succeed())

			// Wait for controller to be ready and CRD to be available
			time.Sleep(10 * time.Second)
			Expect(utils.WaitForCRD()).To(Succeed())

			By("creating a breakglass resource with short duration")
			breakglass := &accessv1alpha1.Breakglass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-breakglass-expire",
					Namespace: namespace,
				},
				Spec: accessv1alpha1.BreakglassSpec{
					Subjects: []accessv1alpha1.SubjectRef{{
						Kind: "User",
						Name: "test-user-expire",
					}},
					AccessPolicy: &accessv1alpha1.AccessPolicy{
						Rules: []accessv1alpha1.AccessRule{{
							Actions:    []accessv1alpha1.Action{"get", "list"},
							Resources:  []string{"pods", "deployments"},
							Namespaces: []string{"default"},
						}},
					},
					ApprovalRequired: false,
					Duration:         &metav1.Duration{Duration: time.Hour},
					Justification:    "E2E test expiration",
				},
			}

			Expect(k8sClient.Create(ctx, breakglass)).To(Succeed())

			By("waiting for breakglass to be created and active")
			Eventually(func() error {
				var bg accessv1alpha1.Breakglass
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-breakglass-expire", Namespace: namespace}, &bg); err != nil {
					return err
				}

				if bg.Status.Phase == "" {
					return fmt.Errorf("state not set")
				}

				if bg.Status.Phase != "Active" {
					return fmt.Errorf("breakglass not active yet")
				}

				return nil
			}, time.Minute, time.Second).Should(Succeed())

			By("waiting for breakglass to expire")
			Eventually(func() error {
				var bg accessv1alpha1.Breakglass
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-breakglass-expire", Namespace: namespace}, &bg); err != nil {
					return err
				}

				// Check if expired
				if bg.Status.Phase == "" {
					return fmt.Errorf("state not set")
				}

				if bg.Status.Phase != "Expired" {
					return fmt.Errorf("expected state Expired, got: %s", bg.Status.Phase)
				}

				// Find expired condition
				var expiredCondition *metav1.Condition
				for i := range bg.Status.Conditions {
					if bg.Status.Conditions[i].Type == conditions.Expired.String() {
						expiredCondition = &bg.Status.Conditions[i]
						break
					}
				}

				if expiredCondition == nil {
					return fmt.Errorf("expired condition not found")
				}

				if expiredCondition.Status != metav1.ConditionTrue {
					return fmt.Errorf("expired condition not true, got: %s", expiredCondition.Status)
				}

				if expiredCondition.Reason != conditions.AccessExpired.String() {
					return fmt.Errorf("expected reason %s, got: %s", conditions.AccessExpired.String(), expiredCondition.Reason)
				}

				return nil
			}, time.Minute*3, time.Second*5).Should(Succeed())

			By("cleaning up test breakglass resource")
			Expect(k8sClient.Delete(ctx, breakglass)).To(Succeed())

			By("cleaning up Skaffold deployment")
			utils.CleanupSkaffoldDeployment("dev")
		})
	})
})
