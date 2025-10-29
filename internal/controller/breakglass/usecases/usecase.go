package usecases

import (
	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
)

// Kind describes the scheduling behaviour for a Breakglass request.
type Kind string

const (
	OneShotUnlimited   Kind = "one-shot-unlimited"
	OneShotFinite      Kind = "one-shot-finite"
	RecurringUnlimited Kind = "recurring-unlimited"
	RecurringFinite    Kind = "recurring-finite"
)

// Determine returns the inferred scheduling behaviour for the given Breakglass.
func Determine(bg *accessv1alpha1.Breakglass) Kind {
	if bg == nil {
		return OneShotUnlimited
	}

	schedule := bg.Spec.Schedule

	// No cron defined → one-shot.
	if schedule.Cron == "" {
		if schedule.Duration.Duration <= 0 {
			return OneShotUnlimited
		}
		return OneShotFinite
	}

	// Cron defined → recurring.
	if schedule.MaxActivations != nil {
		return RecurringFinite
	}
	return RecurringUnlimited
}

// MaxActivationsReached reports whether the Breakglass has consumed its allowed activations.
func MaxActivationsReached(bg *accessv1alpha1.Breakglass) bool {
	if bg == nil || bg.Spec.Schedule.MaxActivations == nil {
		return false
	}
	return bg.Status.ActivationCount >= *bg.Spec.Schedule.MaxActivations
}

// HasFutureActivations reports whether the schedule indicates more work after the current activation.
func HasFutureActivations(bg *accessv1alpha1.Breakglass) bool {
	switch Determine(bg) {
	case RecurringUnlimited:
		return bg.Status.NextActivationAt != nil
	case RecurringFinite:
		if MaxActivationsReached(bg) {
			return false
		}
		return bg.Status.NextActivationAt != nil
	default:
		return false
	}
}
