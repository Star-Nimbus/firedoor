package e2e

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/conditions"
)

var _ = Describe("E2E Test Helpers", func() {
	Describe("randomName", func() {
		It("should generate valid names", func() {
			name1 := randomName("test")
			name2 := randomName("test")

			Expect(name1).To(HavePrefix("test-"))
			Expect(name2).To(HavePrefix("test-"))
			Expect(name1).NotTo(Equal(name2)) // Should be different due to random suffix
		})
	})

	Describe("newBreakglass", func() {
		It("should create a valid breakglass with defaults", func() {
			bg := newBreakglass("test-bg")

			Expect(bg.Name).To(Equal("test-bg"))
			Expect(bg.Namespace).To(Equal(operatorNamespace))
			Expect(bg.APIVersion).To(Equal("access.cloudnimbus.io/v1alpha1"))
			Expect(bg.Kind).To(Equal("Breakglass"))
			Expect(bg.Spec.Justification).To(Equal("e2e test"))
			Expect(bg.Spec.Duration.Duration).To(Equal(time.Minute))
			Expect(bg.Spec.AccessPolicy).NotTo(BeNil())
			Expect(len(bg.Spec.AccessPolicy.Rules)).To(Equal(1))
		})

		It("should apply user option", func() {
			bg := newBreakglass("test-bg", withUser("test-user"))

			Expect(len(bg.Spec.Subjects)).To(Equal(1))
			Expect(bg.Spec.Subjects[0].Kind).To(Equal("User"))
			Expect(bg.Spec.Subjects[0].Name).To(Equal("test-user"))
		})

		It("should apply duration option", func() {
			duration := 30 * time.Second
			bg := newBreakglass("test-bg", withDuration(duration))

			Expect(bg.Spec.Duration.Duration).To(Equal(duration))
		})

		It("should apply multiple options", func() {
			duration := 45 * time.Second
			bg := newBreakglass("test-bg", withUser("test-user"), withDuration(duration))

			Expect(len(bg.Spec.Subjects)).To(Equal(1))
			Expect(bg.Spec.Subjects[0].Name).To(Equal("test-user"))
			Expect(bg.Spec.Duration.Duration).To(Equal(duration))
		})
	})

	Describe("defaultRule", func() {
		It("should return a valid access rule", func() {
			rule := defaultRule()

			// Convert actions to strings for comparison
			var actionStrings []string
			for _, action := range rule.Actions {
				actionStrings = append(actionStrings, string(action))
			}

			Expect(actionStrings).To(ContainElements("get", "list"))
			Expect(rule.Resources).To(ContainElements("pods"))
		})
	})

	Describe("hasCondition", func() {
		It("should find matching condition", func() {
			bg := &accessv1alpha1.Breakglass{
				Status: accessv1alpha1.BreakglassStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Approved",
							Status: metav1.ConditionTrue,
						},
						{
							Type:   "Denied",
							Status: metav1.ConditionFalse,
						},
					},
				},
			}

			Expect(hasCondition(bg, conditions.Approved, metav1.ConditionTrue)).To(BeTrue())
			Expect(hasCondition(bg, conditions.Denied, metav1.ConditionFalse)).To(BeTrue())
			Expect(hasCondition(bg, conditions.Approved, metav1.ConditionFalse)).To(BeFalse())
			Expect(hasCondition(bg, conditions.Expired, metav1.ConditionTrue)).To(BeFalse())
		})
	})
})
