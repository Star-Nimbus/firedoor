package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
)

var _ = Describe("RecurringBreakglassManager", func() {
	var m *RecurringBreakglassManager
	BeforeEach(func() { m = NewRecurringBreakglassManager(time.UTC) })

	// --------------------------------------------------------------------- //
	//  CalculateNextActivation                                              //
	// --------------------------------------------------------------------- //
	DescribeTable("CalculateNextActivation",
		func(schedule string, from, want time.Time) {
			next, err := m.CalculateNextActivation(schedule, from)
			Expect(err).NotTo(HaveOccurred())
			Expect(next).To(Equal(want))
		},
		Entry("daily 09:00", "0 9 * * *",
			time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)),
		Entry("weekly Mon 09:00", "0 9 * * 1",
			time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), // Mon 10:00
			time.Date(2024, 1, 8, 9, 0, 0, 0, time.UTC)), // next Mon
	)

	// --------------------------------------------------------------------- //
	//  ShouldActivateRecurring (skip-missed logic)                          //
	// --------------------------------------------------------------------- //
	Context("ShouldActivateRecurring", func() {
		It("initialises NextActivationAt then returns false", func() {
			bg := &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Recurring:          true,
					RecurrenceSchedule: "0 9 * * *",
				},
			}
			Expect(m.ShouldActivateRecurring(bg)).To(BeFalse())
			Expect(bg.Status.NextActivationAt).NotTo(BeNil())
		})

		It("skips missed windows and updates NextActivationAt", func() {
			past := metav1.NewTime(time.Now().Add(-2 * time.Minute))
			bg := &accessv1alpha1.Breakglass{
				Spec: accessv1alpha1.BreakglassSpec{
					Recurring:          true,
					RecurrenceSchedule: "0 9 * * *",
				},
				Status: accessv1alpha1.BreakglassStatus{NextActivationAt: &past},
			}
			Expect(m.ShouldActivateRecurring(bg)).To(BeFalse())
			Expect(bg.Status.NextActivationAt.Time).Should(BeTemporally(">", past.Time))
		})
	})

	// --------------------------------------------------------------------- //
	//  Transition helpers minimal sanity                                   //
	// --------------------------------------------------------------------- //
	It("transitions through pending → active with correct expiry", func() {
		bg := &accessv1alpha1.Breakglass{
			Spec: accessv1alpha1.BreakglassSpec{
				Recurring:          true,
				RecurrenceSchedule: "0 9 * * *",
				Duration:           &metav1.Duration{Duration: 30 * time.Minute},
			},
		}

		Expect(m.TransitionToRecurringPending(bg)).To(Succeed())
		Expect(bg.Status.Phase).To(Equal(accessv1alpha1.PhaseRecurringPending))
		Expect(bg.Status.NextActivationAt).NotTo(BeNil())

		Expect(m.TransitionToRecurringActive(bg)).To(Succeed())
		Expect(bg.Status.Phase).To(Equal(accessv1alpha1.PhaseRecurringActive))
		Expect(bg.Status.GrantedAt).NotTo(BeNil())
		Expect(bg.Status.ExpiresAt).NotTo(BeNil())
		Expect(bg.Status.ExpiresAt.Sub(bg.Status.GrantedAt.Time)).
			To(Equal(30 * time.Minute))
	})
})
