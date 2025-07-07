package controller

import (
	"fmt"
	"math"
	"strings"
	"time"

	cron "github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/telemetry"
)

const defaultDuration = time.Hour // fallback when spec.duration is nil
const maxRequeue = 24 * time.Hour // never sleep longer than this

// RecurringBreakglassManager encapsulates cron parsing / state updates.
// NOTE: Callers must persist bg.Status after any mutating helper.
type RecurringBreakglassManager struct {
	cronParser cron.Parser
	loc        *time.Location
}

// NewRecurringBreakglassManager returns a manager that interprets schedules
// in the provided *loc* (defaults to UTC if nil).
func NewRecurringBreakglassManager(loc *time.Location) *RecurringBreakglassManager {
	if loc == nil {
		loc = time.UTC
	}
	return &RecurringBreakglassManager{
		cronParser: cron.NewParser(
			cron.Second | cron.Minute | cron.Hour |
				cron.Dom | cron.Month | cron.DowOptional | cron.Descriptor,
		),
		loc: loc,
	}
}

// ────────────────────────────────────────────────────────────────────────────
//  Public helpers
// ────────────────────────────────────────────────────────────────────────────

// IsRecurringBreakglass reports whether spec marks this as recurring.
func (r *RecurringBreakglassManager) IsRecurringBreakglass(bg *accessv1alpha1.Breakglass) bool {
	return bg.Spec.Recurring && bg.Spec.RecurrenceSchedule != ""
}

// normalise ensures we always feed a 6-field cron string to robfig/cron.
func (r *RecurringBreakglassManager) normalise(schedule string) string {
	if len(strings.Fields(schedule)) == 5 {
		return "0 " + schedule
	}
	return schedule
}

// ValidateRecurrenceSchedule returns error if schedule is empty or unparsable.
func (r *RecurringBreakglassManager) ValidateRecurrenceSchedule(s string) error {
	s = r.normalise(s)
	if s == "" {
		return fmt.Errorf("recurrence schedule cannot be empty")
	}
	_, err := r.cronParser.Parse(s)
	return err
}

// CalculateNextActivation returns the first activation strictly AFTER *from*.
func (r *RecurringBreakglassManager) CalculateNextActivation(s string, from time.Time) (time.Time, error) {
	s = r.normalise(s)
	if err := r.ValidateRecurrenceSchedule(s); err != nil {
		return time.Time{}, err
	}
	cs, _ := r.cronParser.Parse(s)
	return cs.Next(from.In(r.loc)), nil
}

// ShouldActivateRecurring mutates bg.Status.NextActivationAt if needed and
// returns true when 'now' ≥ scheduled time (after accounting for missed windows).
func (r *RecurringBreakglassManager) ShouldActivateRecurring(bg *accessv1alpha1.Breakglass) bool {
	if !r.IsRecurringBreakglass(bg) {
		return false
	}
	now := time.Now().In(r.loc)

	// Initialise NextActivationAt if missing
	if bg.Status.NextActivationAt == nil {
		if next, err := r.CalculateNextActivation(bg.Spec.RecurrenceSchedule, now); err == nil {
			bg.Status.NextActivationAt = &metav1.Time{Time: next}
		}
		return false
	}

	cs, err := r.cronParser.Parse(bg.Spec.RecurrenceSchedule)
	if err != nil {
		return false // invalid schedule -> ignore
	}
	next := bg.Status.NextActivationAt.Time.In(r.loc)

	// Fast-forward past missed windows
	for next.Before(now) {
		next = cs.Next(next)
		bg.Status.NextActivationAt = &metav1.Time{Time: next}
		telemetry.RecordOperation(
			telemetry.OpReconcile, telemetry.ResultError,
			telemetry.ComponentController, telemetry.RoleUnknown, "recurring_skip",
		)
	}

	return !now.Before(next) // activate if now == next (cron precision is ≥1s)
}

// UpdateRecurringStatus increments counters and schedules the next run.
func (r *RecurringBreakglassManager) UpdateRecurringStatus(bg *accessv1alpha1.Breakglass) error {
	if !r.IsRecurringBreakglass(bg) {
		return fmt.Errorf("breakglass is not configured for recurring access")
	}

	now := time.Now().In(r.loc)
	if bg.Status.ActivationCount < math.MaxInt32 {
		bg.Status.ActivationCount++
	}
	bg.Status.LastActivationAt = &metav1.Time{Time: now}

	next, err := r.CalculateNextActivation(bg.Spec.RecurrenceSchedule, now)
	if err != nil {
		return err
	}
	bg.Status.NextActivationAt = &metav1.Time{Time: next}
	return nil
}

// GetRequeueTimeForRecurring returns a bounded requeue interval.
func (r *RecurringBreakglassManager) GetRequeueTimeForRecurring(bg *accessv1alpha1.Breakglass) (time.Duration, error) {
	if !r.IsRecurringBreakglass(bg) {
		return 0, fmt.Errorf("breakglass is not configured for recurring access")
	}

	var target time.Time
	switch {
	case bg.Status.Phase == accessv1alpha1.PhaseRecurringActive && bg.Status.ExpiresAt != nil:
		target = bg.Status.ExpiresAt.Time
	case bg.Status.Phase == accessv1alpha1.PhaseRecurringPending && bg.Status.NextActivationAt != nil:
		target = bg.Status.NextActivationAt.Time
	default:
		return time.Minute, nil
	}

	d := time.Until(target)
	if d > maxRequeue {
		d = maxRequeue
	}
	if d < 5*time.Second {
		d = 5 * time.Second // safety minimum
	}
	return d, nil
}

// TransitionToRecurringPending sets phase + first NextActivationAt.
func (r *RecurringBreakglassManager) TransitionToRecurringPending(bg *accessv1alpha1.Breakglass) error {
	if !r.IsRecurringBreakglass(bg) {
		return fmt.Errorf("breakglass is not configured for recurring access")
	}
	bg.Status.Phase = accessv1alpha1.PhaseRecurringPending
	if bg.Status.NextActivationAt == nil {
		if next, err := r.CalculateNextActivation(bg.Spec.RecurrenceSchedule, time.Now().In(r.loc)); err == nil {
			bg.Status.NextActivationAt = &metav1.Time{Time: next}
		} else {
			return err
		}
	}
	return nil
}

// TransitionToRecurringActive sets phase, GrantedAt, and ExpiresAt.
func (r *RecurringBreakglassManager) TransitionToRecurringActive(bg *accessv1alpha1.Breakglass) error {
	if !r.IsRecurringBreakglass(bg) {
		return fmt.Errorf("breakglass is not configured for recurring access")
	}
	bg.Status.Phase = accessv1alpha1.PhaseRecurringActive

	now := metav1.NewTime(time.Now().In(r.loc))
	bg.Status.GrantedAt = &now

	dur := defaultDuration
	if bg.Spec.Duration != nil {
		dur = bg.Spec.Duration.Duration
	}
	exp := metav1.NewTime(now.Time.Add(dur))
	bg.Status.ExpiresAt = &exp
	return nil
}
