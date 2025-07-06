package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/conditions"
	"github.com/cloud-nimbus/firedoor/internal/constants"
)

var _ = Describe("BreakglassOperator", func() {
	var (
		ctx        context.Context
		operator   BreakglassOperator
		mockClient client.Client
		recorder   record.EventRecorder
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		Expect(accessv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(rbacv1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		// Use a simple mock client instead of fake client
		mockClient = &MockClient{}
		recorder = &record.FakeRecorder{}
		operator = NewBreakglassOperator(mockClient, recorder)
	})

	Describe("GrantAccess", func() {
		Context("when validation succeeds", func() {
			var bg *accessv1alpha1.Breakglass

			BeforeEach(func() {
				bg = &accessv1alpha1.Breakglass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-breakglass",
						Namespace: "default",
					},
					Spec: accessv1alpha1.BreakglassSpec{
						User:            "test-user",
						Namespace:       "default",
						Role:            "test-role",
						DurationMinutes: 60,
						Approved:        true,
					},
					Status: accessv1alpha1.BreakglassStatus{
						ApprovedBy: constants.ControllerIdentity,
					},
				}
			})

			It("should grant access successfully", func() {
				result, err := operator.GrantAccess(ctx, bg)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))
				Expect(*bg.Status.Phase).To(Equal(accessv1alpha1.PhaseActive))
				Expect(bg.Status.GrantedAt).NotTo(BeNil())
				Expect(bg.Status.ExpiresAt).NotTo(BeNil())

				// Verify conditions
				Expect(bg.Status.Conditions).To(HaveLen(3))

				approvedCondition := findCondition(bg.Status.Conditions, conditions.Approved.String())
				Expect(approvedCondition).NotTo(BeNil())
				Expect(approvedCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(approvedCondition.Reason).To(Equal(conditions.AccessGranted.String()))

				activeCondition := findCondition(bg.Status.Conditions, conditions.Active.String())
				Expect(activeCondition).NotTo(BeNil())
				Expect(activeCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(activeCondition.Reason).To(Equal(conditions.AccessActive.String()))
			})
		})

		Context("when validation fails", func() {
			var bg *accessv1alpha1.Breakglass

			BeforeEach(func() {
				bg = &accessv1alpha1.Breakglass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-breakglass",
						Namespace: "default",
					},
					Spec: accessv1alpha1.BreakglassSpec{
						Namespace:       "default",
						Role:            "test-role",
						DurationMinutes: 60,
						Approved:        true,
					},
				}
			})

			It("should deny access and set appropriate conditions", func() {
				result, err := operator.GrantAccess(ctx, bg)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
				Expect(*bg.Status.Phase).To(Equal(accessv1alpha1.PhaseDenied))
				Expect(bg.Status.ApprovedBy).To(Equal(constants.ControllerIdentity))

				// Verify conditions
				Expect(bg.Status.Conditions).To(HaveLen(2))

				deniedCondition := findCondition(bg.Status.Conditions, conditions.Denied.String())
				Expect(deniedCondition).NotTo(BeNil())
				Expect(deniedCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(deniedCondition.Reason).To(Equal(conditions.InvalidRequest.String()))
				Expect(deniedCondition.Message).To(ContainSubstring("Missing user or group"))

				approvedCondition := findCondition(bg.Status.Conditions, conditions.Approved.String())
				Expect(approvedCondition).NotTo(BeNil())
				Expect(approvedCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(approvedCondition.Message).To(Equal(conditions.RequestDeniedDueToMissingUserOrGroup.String()))
			})
		})
	})

	Describe("RevokeAccess", func() {
		Context("when revocation succeeds", func() {
			var bg *accessv1alpha1.Breakglass

			BeforeEach(func() {
				phase := accessv1alpha1.PhaseActive
				bg = &accessv1alpha1.Breakglass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-breakglass",
						Namespace: "default",
					},
					Spec: accessv1alpha1.BreakglassSpec{
						Namespace: "default",
						Role:      "test-role",
					},
					Status: accessv1alpha1.BreakglassStatus{
						Phase: &phase,
					},
				}
			})

			It("should revoke access successfully", func() {
				result, err := operator.RevokeAccess(ctx, bg)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
				Expect(*bg.Status.Phase).To(Equal(accessv1alpha1.PhaseExpired))

				// Verify conditions
				Expect(bg.Status.Conditions).To(HaveLen(2))

				expiredCondition := findCondition(bg.Status.Conditions, conditions.Expired.String())
				Expect(expiredCondition).NotTo(BeNil())
				Expect(expiredCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(expiredCondition.Reason).To(Equal(conditions.AccessExpired.String()))
				Expect(expiredCondition.Message).To(Equal(conditions.BreakglassAccessExpiredAndRevoked.String()))

				activeCondition := findCondition(bg.Status.Conditions, conditions.Active.String())
				Expect(activeCondition).NotTo(BeNil())
				Expect(activeCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(activeCondition.Message).To(Equal(conditions.AccessIsNoLongerActive.String()))
			})
		})
	})
})

var _ = Describe("resolveSubject", func() {
	Context("when user is specified", func() {
		It("should return user subject", func() {
			bg := &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					User: "test-user",
				},
			}

			subject, err := resolveSubject(bg)

			Expect(err).NotTo(HaveOccurred())
			Expect(subject.Kind).To(Equal(rbacv1.UserKind))
			Expect(subject.Name).To(Equal("test-user"))
			Expect(subject.APIGroup).To(Equal(rbacv1.GroupName))
		})
	})

	Context("when group is specified", func() {
		It("should return group subject", func() {
			bg := &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Group: "test-group",
				},
			}

			subject, err := resolveSubject(bg)

			Expect(err).NotTo(HaveOccurred())
			Expect(subject.Kind).To(Equal(rbacv1.GroupKind))
			Expect(subject.Name).To(Equal("test-group"))
			Expect(subject.APIGroup).To(BeEmpty())
		})
	})

	Context("when neither user nor group is specified", func() {
		It("should return error", func() {
			bg := &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{},
			}

			subject, err := resolveSubject(bg)

			Expect(err).To(HaveOccurred())
			Expect(subject).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("no user or group provided"))
		})
	})
})

// MockClient is a simple mock implementation for testing
type MockClient struct{}

func (m *MockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return nil
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

func (m *MockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return nil
}

func (m *MockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return nil
}

func (m *MockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return nil
}

func (m *MockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

func (m *MockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

func (m *MockClient) Status() client.StatusWriter {
	return &MockStatusWriter{}
}

func (m *MockClient) Scheme() *runtime.Scheme {
	return runtime.NewScheme()
}

func (m *MockClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (m *MockClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (m *MockClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return false, nil
}

func (m *MockClient) SubResource(subResource string) client.SubResourceClient {
	return &MockSubResourceClient{}
}

// MockStatusWriter is a simple mock implementation for testing
type MockStatusWriter struct{}

func (m *MockStatusWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return nil
}

func (m *MockStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return nil
}

func (m *MockStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return nil
}

// MockSubResourceClient is a simple mock implementation for testing
type MockSubResourceClient struct{}

func (m *MockSubResourceClient) Get(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceGetOption) error {
	return nil
}

func (m *MockSubResourceClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return nil
}

func (m *MockSubResourceClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return nil
}

func (m *MockSubResourceClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return nil
}

// Helper function to find a condition by type
func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
