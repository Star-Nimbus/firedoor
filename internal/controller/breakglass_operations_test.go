package controller

import (
	"context"
	"time"

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
	"github.com/cloud-nimbus/firedoor/internal/alerting"
	"github.com/cloud-nimbus/firedoor/internal/conditions"
	"github.com/cloud-nimbus/firedoor/internal/config"
	"github.com/cloud-nimbus/firedoor/internal/constants"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("BreakglassOperator", func() {
	var (
		ctx        context.Context
		operator   BreakglassOperator
		mockClient *MockClient
		recorder   record.EventRecorder
		scheme     *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(accessv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(rbacv1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		// Use a simple mock client instead of fake client
		mockClient = &MockClient{scheme: scheme}
		recorder = &record.FakeRecorder{}
		// Create a disabled Alertmanager service for testing
		alertService := alerting.NewAlertmanagerService(&config.AlertmanagerConfig{Enabled: false}, recorder)
		operator = NewBreakglassOperator(mockClient, recorder, alertService, false)
	})

	Describe("GrantAccess", func() {
		Context("when validation succeeds", func() {
			var bg *accessv1alpha1.Breakglass

			BeforeEach(func() {
				bg = newBG("test-breakglass")
				bg.Spec = accessv1alpha1.BreakglassSpec{
					AccessPolicy: &accessv1alpha1.AccessPolicy{
						Rules: []accessv1alpha1.AccessRule{{
							Actions:    []accessv1alpha1.Action{"get", "list"},
							Resources:  []string{"pods", "deployments"},
							Namespaces: []string{"default"},
						}},
					},
					ApprovalRequired: false,
					Duration:         &metav1.Duration{Duration: time.Minute},
					Justification:    "Test breakglass",
					Subjects:         []accessv1alpha1.SubjectRef{{Kind: rbacv1.UserKind, Name: "test-user"}},
				}
			})

			It("should grant access successfully", func() {
				result, err := operator.GrantAccess(ctx, bg)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))
				Expect(bg.Status.Phase).To(Equal(accessv1alpha1.PhaseActive))
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
				bg = newBG("test-breakglass")
				bg.Spec = accessv1alpha1.BreakglassSpec{
					AccessPolicy: &accessv1alpha1.AccessPolicy{
						Rules: []accessv1alpha1.AccessRule{{
							Actions:    []accessv1alpha1.Action{"get", "list"},
							Resources:  []string{"pods", "deployments"},
							Namespaces: []string{"default"},
						}},
					},
					ApprovalRequired: false,
					Duration:         &metav1.Duration{Duration: time.Minute},
					Justification:    "Test breakglass invalid",
					// No subjects - this should cause validation to fail
					Subjects: []accessv1alpha1.SubjectRef{},
				}
			})

			It("should deny access and set appropriate conditions", func() {
				result, err := operator.GrantAccess(ctx, bg)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(bg.Status.Phase).To(Equal(accessv1alpha1.PhaseDenied))
				Expect(bg.Status.ApprovedBy).To(Equal(constants.ControllerIdentity))

				// Verify conditions
				Expect(bg.Status.Conditions).To(HaveLen(2))

				deniedCondition := findCondition(bg.Status.Conditions, conditions.Denied.String())
				Expect(deniedCondition).NotTo(BeNil())
				Expect(deniedCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(deniedCondition.Reason).To(Equal(conditions.InvalidRequest.String()))
				Expect(deniedCondition.Message).To(ContainSubstring("Missing subjects"))

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
				bg = newBG("test-breakglass")
				bg.Spec = accessv1alpha1.BreakglassSpec{
					AccessPolicy: &accessv1alpha1.AccessPolicy{
						Rules: []accessv1alpha1.AccessRule{{
							Actions:    []accessv1alpha1.Action{"get", "list"},
							Resources:  []string{"pods", "deployments"},
							Namespaces: []string{"default"},
						}},
					},
					ApprovalRequired: false,
					Duration:         &metav1.Duration{Duration: time.Minute},
					Justification:    "Test breakglass",
					Subjects:         []accessv1alpha1.SubjectRef{{Kind: rbacv1.UserKind, Name: "test-user"}},
				}
			})

			It("should revoke access successfully", func() {
				result, err := operator.RevokeAccess(ctx, bg)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
				Expect(bg.Status.Phase).To(Equal(accessv1alpha1.PhaseExpired))

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

	Describe("ClusterRole creation with custom rules", func() {
		It("should create a ClusterRole and ClusterRoleBinding for custom rules", func() {
			bg := newBG("custom-clusterrole-bg")
			bg.Spec = accessv1alpha1.BreakglassSpec{
				Subjects:     []accessv1alpha1.SubjectRef{{Kind: rbacv1.UserKind, Name: "test-user"}},
				ClusterRoles: []string{"custom-breakglass-role"},
				AccessPolicy: &accessv1alpha1.AccessPolicy{
					Rules: []accessv1alpha1.AccessRule{{
						Actions:   []accessv1alpha1.Action{"get", "list"},
						Resources: []string{"pods"},
						APIGroups: []string{""},
					}},
				},
				ApprovalRequired: false,
				Duration:         &metav1.Duration{Duration: time.Minute},
				Justification:    "Test custom clusterrole",
			}

			testMockClient := &MockClient{scheme: scheme}
			recorder := &record.FakeRecorder{}
			// Create a disabled Alertmanager service for testing
			alertService := alerting.NewAlertmanagerService(&config.AlertmanagerConfig{Enabled: false}, recorder)
			operator := NewBreakglassOperator(testMockClient, recorder, alertService, false)

			_, err := operator.GrantAccess(context.Background(), bg)
			Expect(err).NotTo(HaveOccurred())

			// Check that the ClusterRole and ClusterRoleBinding were created
			createdClusterRole := testMockClient.GetCreatedObject("ClusterRole", "custom-breakglass-role")
			Expect(createdClusterRole).NotTo(BeNil())
			cr, ok := createdClusterRole.(*rbacv1.ClusterRole)
			Expect(ok).To(BeTrue())
			Expect(cr.Rules).ToNot(BeEmpty())
			Expect(cr.Rules[0].Verbs).To(ContainElements("get", "list"))
			Expect(cr.Rules[0].Resources).To(ContainElements("pods"))

			createdClusterRoleBinding := testMockClient.GetCreatedObject("ClusterRoleBinding", "breakglass-custom-clusterrole-bg-custom-breakglass-role")
			Expect(createdClusterRoleBinding).NotTo(BeNil())
			crb, ok := createdClusterRoleBinding.(*rbacv1.ClusterRoleBinding)
			Expect(ok).To(BeTrue())
			Expect(crb.Subjects).To(HaveLen(1))
			Expect(crb.RoleRef.Name).To(Equal("custom-breakglass-role"))
		})
	})
})

var _ = Describe("resolveSubjects", func() {
	Context("when user is specified", func() {
		It("should return user subject", func() {
			bg := newBG("test-breakglass")
			bg.Spec = accessv1alpha1.BreakglassSpec{
				AccessPolicy: &accessv1alpha1.AccessPolicy{
					Rules: []accessv1alpha1.AccessRule{{
						Actions:    []accessv1alpha1.Action{"get", "list"},
						Resources:  []string{"pods", "deployments"},
						Namespaces: []string{"default"},
					}},
				},
				ApprovalRequired: false,
				Duration:         &metav1.Duration{Duration: time.Minute},
				Justification:    "Test breakglass",
				Subjects:         []accessv1alpha1.SubjectRef{{Kind: rbacv1.UserKind, Name: "test-user"}},
			}

			subjects, err := resolveSubjects(context.Background(), bg)

			Expect(err).NotTo(HaveOccurred())
			Expect(subjects).To(HaveLen(1))
			Expect(subjects[0].Kind).To(Equal(rbacv1.UserKind))
			Expect(subjects[0].Name).To(Equal("test-user"))
			Expect(subjects[0].APIGroup).To(Equal(rbacv1.GroupName))
		})
	})

	Context("when group is specified", func() {
		It("should return group subject", func() {
			bg := newBG("test-breakglass")
			bg.Spec = accessv1alpha1.BreakglassSpec{
				AccessPolicy: &accessv1alpha1.AccessPolicy{
					Rules: []accessv1alpha1.AccessRule{{
						Actions:    []accessv1alpha1.Action{"get", "list"},
						Resources:  []string{"pods", "deployments"},
						Namespaces: []string{"default"},
					}},
				},
				ApprovalRequired: false,
				Duration:         &metav1.Duration{Duration: time.Minute},
				Justification:    "Test breakglass",
				Subjects:         []accessv1alpha1.SubjectRef{{Kind: rbacv1.GroupKind, Name: "test-group"}},
			}

			subjects, err := resolveSubjects(context.Background(), bg)

			Expect(err).NotTo(HaveOccurred())
			Expect(subjects).To(HaveLen(1))
			Expect(subjects[0].Kind).To(Equal(rbacv1.GroupKind))
			Expect(subjects[0].Name).To(Equal("test-group"))
			Expect(subjects[0].APIGroup).To(BeEmpty())
		})
	})

	Context("when neither user nor group is specified", func() {
		It("should return error", func() {
			bg := newBG("test-breakglass")
			bg.Spec = accessv1alpha1.BreakglassSpec{
				AccessPolicy: &accessv1alpha1.AccessPolicy{
					Rules: []accessv1alpha1.AccessRule{{
						Actions:    []accessv1alpha1.Action{"get", "list"},
						Resources:  []string{"pods", "deployments"},
						Namespaces: []string{"default"},
					}},
				},
				ApprovalRequired: false,
				Duration:         &metav1.Duration{Duration: time.Minute},
				Justification:    "Test breakglass",
			}

			subjects, err := resolveSubjects(context.Background(), bg)

			Expect(err).To(HaveOccurred())
			Expect(subjects).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("no subjects provided"))
		})
	})
})

// MockClient is a simple mock implementation for testing
type MockClient struct {
	createdObjects map[string]map[string]client.Object // kind -> name -> object
	scheme         *runtime.Scheme
}

func (m *MockClient) ensureMap() {
	if m.createdObjects == nil {
		m.createdObjects = make(map[string]map[string]client.Object)
	}
}

func (m *MockClient) ensureKind(kind string) {
	m.ensureMap()
	if m.createdObjects[kind] == nil {
		m.createdObjects[kind] = make(map[string]client.Object)
	}
}

func (m *MockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	kind := kindOf(obj)
	m.ensureKind(kind)
	if stored, ok := m.createdObjects[kind][key.Name]; ok {
		unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(stored)
		if err != nil {
			return err
		}
		_ = runtime.DefaultUnstructuredConverter.FromUnstructured(unstr, obj)
		return nil
	}
	return apierrors.NewNotFound(schema.GroupResource{Resource: kind}, key.Name)
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

func (m *MockClient) Create(ctx context.Context, obj client.Object, _ ...client.CreateOption) error {
	kind := kindOf(obj)
	m.ensureKind(kind)
	m.createdObjects[kind][obj.GetName()] = obj.DeepCopyObject().(client.Object)
	return nil
}

func (m *MockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return nil
}

func (m *MockClient) Update(ctx context.Context, obj client.Object, _ ...client.UpdateOption) error {
	kind := kindOf(obj)
	m.ensureKind(kind)
	m.createdObjects[kind][obj.GetName()] = obj.DeepCopyObject().(client.Object)
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
	return m.scheme
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

func (m *MockClient) GetCreatedObject(kind, name string) client.Object {
	m.ensureKind(kind)
	return m.createdObjects[kind][name]
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

func kindOf(obj runtime.Object) string {
	k := obj.GetObjectKind().GroupVersionKind().Kind
	if k == "" {
		switch obj.(type) {
		case *rbacv1.ClusterRole:
			k = "ClusterRole"
		case *rbacv1.ClusterRoleBinding:
			k = "ClusterRoleBinding"
		case *rbacv1.Role:
			k = "Role"
		case *rbacv1.RoleBinding:
			k = "RoleBinding"
		default:
			k = "Unknown"
		}
	}
	return k
}

func newBG(name string) *accessv1alpha1.Breakglass {
	return &accessv1alpha1.Breakglass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: accessv1alpha1.GroupVersion.String(),
			Kind:       "Breakglass",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}
}
