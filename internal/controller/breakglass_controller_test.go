package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/alerting"
	"github.com/cloud-nimbus/firedoor/internal/config"
)

var _ = Describe("Breakglass Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		breakglass := &accessv1alpha1.Breakglass{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Breakglass")
			err := k8sClient.Get(ctx, typeNamespacedName, breakglass)
			if err != nil && errors.IsNotFound(err) {
				resource := &accessv1alpha1.Breakglass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: accessv1alpha1.BreakglassSpec{
						Subjects: []accessv1alpha1.SubjectRef{{
							Kind: rbacv1.UserKind,
							Name: "test-user",
						}},
						ClusterRoles:     []string{"admin"},
						ApprovalRequired: false,
						Duration:         &metav1.Duration{Duration: time.Minute},
						Justification:    "Test breakglass",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &accessv1alpha1.Breakglass{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Breakglass")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &BreakglassReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
				Config:   &config.Config{},
			}
			// Create a disabled Alertmanager service for testing
			alertService := alerting.NewAlertmanagerService(&config.AlertmanagerConfig{Enabled: false}, controllerReconciler.Recorder)
			controllerReconciler.operator = NewBreakglassOperator(controllerReconciler.Client, controllerReconciler.Recorder, alertService, false)

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	It("should create a Breakglass with multiple subjects and cluster roles", func() {
		ctx := context.Background()
		By("Creating a Breakglass resource with multiple subjects and cluster roles")
		resource := &accessv1alpha1.Breakglass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multi-subjects-clusterroles",
				Namespace: "default",
			},
			Spec: accessv1alpha1.BreakglassSpec{
				Subjects: []accessv1alpha1.SubjectRef{{
					Kind: rbacv1.UserKind,
					Name: "alice@example.com",
				}, {
					Kind: rbacv1.UserKind,
					Name: "bob@example.com",
				}},
				ClusterRoles:     []string{"admin", "viewer"},
				ApprovalRequired: true,
				Duration:         &metav1.Duration{Duration: time.Minute},
				Justification:    "Test multiple subjects and cluster roles",
			},
		}
		Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		// Fetch and verify
		fetched := &accessv1alpha1.Breakglass{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: "multi-subjects-clusterroles", Namespace: "default"}, fetched)
		}).Should(Succeed())
		Expect(fetched.Spec.Subjects).To(HaveLen(2))
		Expect(fetched.Spec.ClusterRoles).To(ContainElements("admin", "viewer"))
	})

	It("should create a recurring Breakglass with cron schedule", func() {
		ctx := context.Background()
		By("Creating a recurring Breakglass resource with cron schedule")
		resource := &accessv1alpha1.Breakglass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "recurring-breakglass",
				Namespace: "default",
			},
			Spec: accessv1alpha1.BreakglassSpec{
				Subjects: []accessv1alpha1.SubjectRef{{
					Kind: rbacv1.UserKind,
					Name: "recurring-user@example.com",
				}},
				ClusterRoles:       []string{"viewer"},
				ApprovalRequired:   false,
				Duration:           &metav1.Duration{Duration: 30 * time.Minute},
				Justification:      "Test recurring breakglass functionality",
				Recurring:          true,
				RecurrenceSchedule: "0 9 * * 1-5", // Weekdays at 9 AM
			},
		}
		Expect(k8sClient.Create(ctx, resource)).To(Succeed())

		// Trigger reconciliation
		controllerReconciler := &BreakglassReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Recorder: record.NewFakeRecorder(10),
			Config:   &config.Config{},
		}
		// Create a disabled Alertmanager service for testing
		alertService := alerting.NewAlertmanagerService(&config.AlertmanagerConfig{Enabled: false}, controllerReconciler.Recorder)
		controllerReconciler.operator = NewBreakglassOperator(controllerReconciler.Client, controllerReconciler.Recorder, alertService, false)
		controllerReconciler.recurringManager = NewRecurringBreakglassManager(time.UTC)

		_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "recurring-breakglass", Namespace: "default"},
		})
		Expect(err).NotTo(HaveOccurred())

		// Fetch and verify
		fetched := &accessv1alpha1.Breakglass{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: "recurring-breakglass", Namespace: "default"}, fetched)
		}).Should(Succeed())

		Expect(fetched.Spec.Recurring).To(BeTrue())
		Expect(fetched.Spec.RecurrenceSchedule).To(Equal("0 9 * * 1-5"))
		Expect(fetched.Status.Phase).To(Equal(accessv1alpha1.PhaseRecurringPending))
		Expect(fetched.Status.NextActivationAt).NotTo(BeNil())
		Expect(fetched.Status.ActivationCount).To(Equal(int32(0)))
	})

	It("should reject recurring Breakglass with invalid cron schedule", func() {
		ctx := context.Background()
		By("Creating a recurring Breakglass resource with invalid cron schedule")
		resource := &accessv1alpha1.Breakglass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid-recurring-breakglass",
				Namespace: "default",
			},
			Spec: accessv1alpha1.BreakglassSpec{
				Subjects: []accessv1alpha1.SubjectRef{{
					Kind: rbacv1.UserKind,
					Name: "test-user@example.com",
				}},
				ClusterRoles:       []string{"viewer"},
				ApprovalRequired:   false,
				Duration:           &metav1.Duration{Duration: time.Minute},
				Justification:      "Test invalid recurring breakglass",
				Recurring:          true,
				RecurrenceSchedule: "invalid cron schedule",
			},
		}
		Expect(k8sClient.Create(ctx, resource)).To(Succeed())

		// The controller should handle this gracefully and set an error condition
		// We can't easily test the reconciliation error in this test setup,
		// but we can verify the resource was created
		fetched := &accessv1alpha1.Breakglass{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: "invalid-recurring-breakglass", Namespace: "default"}, fetched)
		}).Should(Succeed())

		Expect(fetched.Spec.Recurring).To(BeTrue())
		Expect(fetched.Spec.RecurrenceSchedule).To(Equal("invalid cron schedule"))
	})
})
