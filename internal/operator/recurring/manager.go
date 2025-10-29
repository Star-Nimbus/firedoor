/*
Copyright 2024 The Cloud-Nimbus Authors.

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

package recurring

import (
	"context"
	"fmt"
	"time"

	"errors"
	"strings"
	"sync"

	cronv3 "github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	siglog "sigs.k8s.io/controller-runtime/pkg/log"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/controller"
)

// locationCache caches time zone lookups
var locationCache = struct {
	m  map[string]*time.Location
	mu sync.RWMutex
}{m: make(map[string]*time.Location)}

func init() {
	// Pre-warm the cache with time.Local to avoid first-call latency
	locationCache.mu.Lock()
	locationCache.m["Local"] = time.Local
	locationCache.mu.Unlock()
}

func getCachedLocation(name string) (*time.Location, error) {
	locationCache.mu.RLock()
	loc, ok := locationCache.m[name]
	locationCache.mu.RUnlock()
	if ok {
		return loc, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, err
	}
	locationCache.mu.Lock()
	locationCache.m[name] = loc
	locationCache.mu.Unlock()
	return loc, nil
}

// Manager implements the controller.RecurringManager interface.
type Manager struct {
	clock  controller.Clock
	parser cronv3.Parser
}

// New creates a new RecurringManager
func New(clock controller.Clock) *Manager {
	return &Manager{
		clock:  clock,
		parser: cronv3.NewParser(cronv3.Minute | cronv3.Hour | cronv3.Dom | cronv3.Month | cronv3.Dow),
	}
}

// ProcessRecurring processes the next occurrence of a recurring breakglass
func (m *Manager) ProcessRecurring(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	log := logFromContext(ctx)

	// Check activation limits (maxActivations, end time)
	if m.checkActivationLimits(bg, log) {
		return nil
	}

	now := m.clock.Now()
	// One-shot schedules (no cron) still flow through this manager
	if bg.Spec.Schedule.Cron == "" {
		return m.processOneShot(bg, now, log)
	}

	// Validate and parse cron schedule
	sched, loc, err := m.parseAndValidateSchedule(&bg.Spec.Schedule)
	if err != nil {
		log.Error(err, "invalid schedule")
		m.setCondition(
			bg,
			accessv1alpha1.ConditionFailed,
			accessv1alpha1.ReasonRecurringInvalidSchedule,
			err.Error(),
			bg.Generation,
		)
		return err
	}

	now = now.In(loc)
	log.V(3).Info("processing recurring", "now", now, "location", loc)

	if bg.Status.NextActivationAt == nil {
		return m.handleInitialActivation(bg, sched, now, log)
	}

	if bg.Status.GrantedAt != nil && now.Before(bg.Status.GrantedAt.Time) {
		log.V(3).Info("clock skew detected; running activation now", "now", now, "lastGrantedAt", bg.Status.GrantedAt)
	}

	if now.Before(bg.Spec.Schedule.Start.Time) {
		log.V(3).Info("not yet start; skipping activation", "now", now, "start", bg.Spec.Schedule.Start)
		return nil
	}

	if bg.Status.NextActivationAt != nil {
		duration := bg.Spec.Schedule.Duration.Duration
		if duration > 0 {
			windowEnd := bg.Status.NextActivationAt.Time.Add(duration)
			if now.After(windowEnd) {
				next := m.calculateNextActivation(sched, now, &bg.Spec.Schedule)
				if next != nil {
					bg.Status.NextActivationAt = &metav1.Time{Time: *next}
					log.V(3).Info("missed activation window; rescheduled", "nextActivation", next)
					m.setCondition(
						bg,
						accessv1alpha1.ConditionRecurringPending,
						accessv1alpha1.ReasonRecurringScheduled,
						fmt.Sprintf(
							"Recurring breakglass scheduled for next activation at %s",
							next.Format(time.RFC3339),
						),
						bg.Generation,
					)
				}
				return nil
			}
		}
	}

	if m.shouldActivateNow(bg, sched, now, log) {
		log.V(3).Info("recurring breakglass activation due", "now", now)
		m.setCondition(
			bg,
			accessv1alpha1.ConditionRecurringPending,
			accessv1alpha1.ReasonRecurringWaiting,
			fmt.Sprintf("Recurring breakglass activation window open since %s", now.Format(time.RFC3339)),
			bg.Generation,
		)
	}

	return nil
}

// ShouldActivate determines if a recurring breakglass should be activated
func (m *Manager) ShouldActivate(ctx context.Context, bg *accessv1alpha1.Breakglass) bool {
	if bg.Status.NextActivationAt == nil {
		return false
	}
	if !bg.Spec.Schedule.Start.IsZero() && m.clock.Now().Before(bg.Spec.Schedule.Start.Time) {
		return false
	}
	return !m.clock.Now().Before(bg.Status.NextActivationAt.Time)
}

// ShouldDeactivate determines if a recurring breakglass should be deactivated
func (m *Manager) ShouldDeactivate(ctx context.Context, bg *accessv1alpha1.Breakglass) bool {
	if bg.Spec.Schedule.Cron == "" {
		return false
	}
	return false // Recurring breakglasses should never auto-deactivate
}

// OnActivationGranted updates scheduling state after a successful activation grant.
func (m *Manager) OnActivationGranted(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	log := logFromContext(ctx)

	// No recurring schedule defined; nothing to advance.
	if bg.Spec.Schedule.Cron == "" {
		bg.Status.NextActivationAt = nil
		bg.Status.ActivationCount++
		log.V(3).Info("non-recurring breakglass activated; no further scheduling")
		return nil
	}

	sched, loc, err := m.parseAndValidateSchedule(&bg.Spec.Schedule)
	if err != nil {
		return err
	}

	ref := m.clock.Now().In(loc)
	if bg.Status.GrantedAt != nil {
		ref = bg.Status.GrantedAt.Time.In(loc)
	}

	next := m.calculateNextActivation(sched, ref, &bg.Spec.Schedule)
	if next == nil {
		return errors.New("could not calculate next activation time after grant")
	}

	bg.Status.ActivationCount++
	bg.Status.NextActivationAt = &metav1.Time{Time: *next}
	m.setCondition(
		bg,
		accessv1alpha1.ConditionRecurringPending,
		accessv1alpha1.ReasonRecurringScheduled,
		fmt.Sprintf(
			"Recurring breakglass scheduled for next activation at %s",
			next.Format(time.RFC3339),
		),
		bg.Generation,
	)
	log.V(3).Info("scheduled next activation after grant", "nextActivation", next)
	return nil
}

func (m *Manager) processOneShot(bg *accessv1alpha1.Breakglass, now time.Time, log logger) error {
	if bg.Status.ActivationCount > 0 {
		return nil
	}

	target := bg.Spec.Schedule.Start.Time
	if target.IsZero() {
		target = now
	}

	if bg.Status.NextActivationAt == nil {
		bg.Status.NextActivationAt = &metav1.Time{Time: target}
		m.setCondition(
			bg,
			accessv1alpha1.ConditionRecurringPending,
			accessv1alpha1.ReasonRecurringScheduled,
			fmt.Sprintf("Breakglass scheduled for activation at %s", target.Format(time.RFC3339)),
			bg.Generation,
		)
		return nil
	}

	if !now.Before(bg.Status.NextActivationAt.Time) {
		m.setCondition(
			bg,
			accessv1alpha1.ConditionRecurringPending,
			accessv1alpha1.ReasonRecurringWaiting,
			fmt.Sprintf("Breakglass activation window open since %s", bg.Status.NextActivationAt.Time.Format(time.RFC3339)),
			bg.Generation,
		)
	}

	return nil
}

// parseAndValidateSchedule parses and validates the cron schedule, returning the schedule and location
func (m *Manager) parseAndValidateSchedule(
	schedule *accessv1alpha1.ScheduleSpec,
) (cronv3.Schedule, *time.Location, error) {
	if schedule.Cron == "" {
		return nil, time.UTC, nil
	}
	if strings.HasPrefix(schedule.Cron, "@") {
		return nil, time.UTC, fmt.Errorf("@-descriptors are not supported; use 5-field cron")
	}
	fields := strings.Fields(schedule.Cron)
	if len(fields) != 5 {
		return nil, time.UTC, fmt.Errorf("cron schedule must have exactly 5 fields (min hour dom month dow)")
	}
	loc := time.UTC
	if schedule.Location != "" {
		var err error
		loc, err = getCachedLocation(schedule.Location)
		if err != nil {
			return nil, time.UTC, fmt.Errorf("invalid time zone: %w", err)
		}
	}
	parser := cronv3.NewParser(cronv3.Minute | cronv3.Hour | cronv3.Dom | cronv3.Month | cronv3.Dow)
	sched, err := parser.Parse(schedule.Cron)
	if err != nil {
		return nil, loc, fmt.Errorf("invalid cron schedule '%s': %w", schedule.Cron, err)
	}

	return sched, loc, nil
}

// calculateNextActivation calculates the next activation time based on the parsed schedule and schedule spec
func (m *Manager) calculateNextActivation(
	sched cronv3.Schedule,
	now time.Time,
	schedule *accessv1alpha1.ScheduleSpec,
) *time.Time {
	if sched == nil {
		return nil
	}

	candidate := sched.Next(now)
	if !schedule.Start.IsZero() && candidate.Before(schedule.Start.Time) {
		candidate = schedule.Start.Time
	}

	return &candidate
}

// findPrevActivation will find a missing cron up to 1 year. TODO: improve later
func (m *Manager) findPrevActivation(
	sched cronv3.Schedule,
	now time.Time,
) *time.Time {
	lookback := now.AddDate(-1, 0, 0) // 1 year back
	prev := lookback
	for t := sched.Next(lookback); !t.After(now); t = sched.Next(t) {
		prev = t
		if now.Sub(prev) > 370*24*time.Hour {
			break
		}
	}

	if prev.Equal(lookback) {
		return nil
	}

	return &prev
}

// setCondition sets a condition on the breakglass status
func (m *Manager) setCondition(
	bg *accessv1alpha1.Breakglass,
	conditionType accessv1alpha1.BreakglassCondition,
	reason accessv1alpha1.BreakglassConditionReason,
	message string,
	observedGeneration int64,
) {
	condition := metav1.Condition{
		Type:               string(conditionType),
		Status:             metav1.ConditionTrue,
		Reason:             string(reason),
		Message:            message,
		LastTransitionTime: metav1.NewTime(m.clock.Now()),
		ObservedGeneration: observedGeneration,
	}
	meta.SetStatusCondition(&bg.Status.Conditions, condition)
}

// loggerKey is a private type for context logger keys
// This avoids collisions with other context values
// Usage: ctx = context.WithValue(ctx, loggerKey{}, myLogger)
type loggerKey struct{}

// logFromContext returns a logger from context or stdlib fallback
func logFromContext(ctx context.Context) logger {
	if l, ok := ctx.Value(loggerKey{}).(logger); ok {
		return l
	}
	return stdLogger{}
}

type logger interface {
	V(level int) logger
	Info(msg string, keysAndValues ...interface{})
	Error(err error, msg string, keysAndValues ...interface{})
}

type stdLogger struct{}

func (stdLogger) V(level int) logger { return stdLogger{} }
func (stdLogger) Info(msg string, keysAndValues ...interface{}) {
	siglog.Log.Info(msg, keysAndValues...)
}
func (stdLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	siglog.Log.Error(err, msg, keysAndValues...)
}

// checkActivationLimits checks maxActivations and end time
func (m *Manager) checkActivationLimits(bg *accessv1alpha1.Breakglass, log logger) bool {
	schedule := &bg.Spec.Schedule
	if schedule.MaxActivations != nil && bg.Status.ActivationCount >= *schedule.MaxActivations {
		log.V(3).Info("max activations reached; stopping recurrence", "count", bg.Status.ActivationCount)
		bg.Status.NextActivationAt = nil
		m.setCondition(
			bg,
			accessv1alpha1.ConditionFailed,
			accessv1alpha1.ReasonMaxActivationsReached,
			fmt.Sprintf("Maximum activations reached: %d", *schedule.MaxActivations),
			bg.Generation,
		)
		return true
	}

	return false

}

// handleInitialActivation schedules the first activation if needed
func (m *Manager) handleInitialActivation(
	bg *accessv1alpha1.Breakglass,
	sched cronv3.Schedule,
	now time.Time,
	log logger,
) error {
	nextActivation := m.calculateNextActivation(sched, now, &bg.Spec.Schedule)
	if nextActivation == nil {
		log.Error(errors.New("no next activation"), "could not calculate next activation time")
		m.setCondition(
			bg,
			accessv1alpha1.ConditionFailed,
			accessv1alpha1.ReasonRecurringInvalidSchedule,
			"could not calculate next activation time",
			bg.Generation,
		)
		return nil
	}
	prev := m.findPrevActivation(sched, now)
	if prev != nil {
		windowEnd := prev.Add(bg.Spec.Schedule.Duration.Duration)
		if now.After(*prev) && now.Before(windowEnd) {
			bg.Status.NextActivationAt = &metav1.Time{Time: *prev}
			log.V(3).Info("found previous activation window still open", "prev", prev, "windowEnd", windowEnd)
			m.setCondition(
				bg,
				accessv1alpha1.ConditionRecurringPending,
				accessv1alpha1.ReasonRecurringWaiting,
				fmt.Sprintf("Recurring breakglass activation window open since %s", prev.Format(time.RFC3339)),
				bg.Generation,
			)
			return nil
		}
	}

	bg.Status.NextActivationAt = &metav1.Time{Time: *nextActivation}
	log.V(3).Info("scheduled initial activation", "nextActivation", nextActivation)
	m.setCondition(
		bg,
		accessv1alpha1.ConditionRecurringPending,
		accessv1alpha1.ReasonRecurringScheduled,
		fmt.Sprintf(
			"Recurring breakglass scheduled for next activation at %s",
			nextActivation.Format(time.RFC3339),
		),
		bg.Generation,
	)
	return nil
}

// shouldActivateNow returns true if it's time to activate
func (m *Manager) shouldActivateNow(bg *accessv1alpha1.Breakglass, sched cronv3.Schedule, now time.Time, log logger) bool {
	prev := m.findPrevActivation(sched, now)
	if prev != nil {
		windowEnd := prev.Add(bg.Spec.Schedule.Duration.Duration)
		if now.After(*prev) && now.Before(windowEnd) {
			log.V(3).Info("within active window from previous tick",
				"prev", prev, "windowEnd", windowEnd)
			return true
		}
	}

	if bg.Status.NextActivationAt != nil && !now.Before(bg.Status.NextActivationAt.Time) {
		return true
	}
	return false
}
