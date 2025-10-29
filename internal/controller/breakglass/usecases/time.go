package usecases

import (
	"time"

	cronv3 "github.com/robfig/cron/v3"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
)

// Window represents the current activation window for a Breakglass request.
type Window struct {
	Start time.Time
	End   time.Time
}

var cronParser = cronv3.NewParser(cronv3.Minute | cronv3.Hour | cronv3.Dom | cronv3.Month | cronv3.Dow)

// CurrentWindow computes the activation window that applies at the provided
// reference time. When the schedule has no finite duration, false is returned.
func CurrentWindow(bg *accessv1alpha1.Breakglass, now time.Time) (Window, bool) {
	if bg == nil {
		return Window{}, false
	}

	duration := bg.Spec.Schedule.Duration.Duration
	if duration <= 0 {
		return Window{}, false
	}

	if bg.Spec.Schedule.Cron == "" {
		start := resolveOneShotStart(bg, now)
		end := start.Add(duration)
		return Window{Start: start, End: end}, true
	}

	sched, loc, err := parseCronSchedule(&bg.Spec.Schedule)
	if err != nil {
		return Window{}, false
	}

	nowLoc := now.In(loc)
	start := findPreviousActivation(sched, nowLoc)

	// Fallback to the recorded next activation if the schedule is ahead of us
	if start.IsZero() && bg.Status.NextActivationAt != nil {
		candidate := bg.Status.NextActivationAt.Time.In(loc)
		if !nowLoc.Before(candidate) {
			start = candidate
		}
	}

	if start.IsZero() {
		return Window{}, false
	}

	end := start.Add(duration)

	return Window{Start: start.UTC(), End: end.UTC()}, true
}

// FinalCompletionTime returns the timestamp that should be recorded when the
// Breakglass completes. If the schedule has no finite duration, false is
// returned.
func FinalCompletionTime(bg *accessv1alpha1.Breakglass, now time.Time) (*time.Time, bool) {
	window, ok := CurrentWindow(bg, now)
	if !ok {
		return nil, false
	}

	end := window.End
	return &end, true
}

func resolveOneShotStart(bg *accessv1alpha1.Breakglass, now time.Time) time.Time {
	if !bg.Spec.Schedule.Start.IsZero() {
		return bg.Spec.Schedule.Start.Time.UTC()
	}

	if bg.Status.NextActivationAt != nil {
		return bg.Status.NextActivationAt.Time.UTC()
	}

	if bg.Status.GrantedAt != nil {
		return bg.Status.GrantedAt.Time.UTC()
	}

	return now.UTC()
}

func parseCronSchedule(schedule *accessv1alpha1.ScheduleSpec) (cronv3.Schedule, *time.Location, error) {
	loc := time.UTC
	if schedule.Location != "" {
		var err error
		loc, err = time.LoadLocation(schedule.Location)
		if err != nil {
			return nil, time.UTC, err
		}
	}

	sched, err := cronParser.Parse(schedule.Cron)
	if err != nil {
		return nil, time.UTC, err
	}

	return sched, loc, nil
}

func findPreviousActivation(sched cronv3.Schedule, now time.Time) time.Time {
	lookback := now.AddDate(-1, 0, 0)
	prev := lookback

	for t := sched.Next(lookback); !t.After(now); t = sched.Next(t) {
		prev = t
		if now.Sub(prev) > 370*24*time.Hour {
			break
		}
	}

	if prev.Equal(lookback) {
		return time.Time{}
	}

	return prev
}
