package breakglass

import (
	"fmt"
	"testing"
	"time"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// validateScheduleSpec validates a schedule specification
func validateScheduleSpec(spec *accessv1alpha1.ScheduleSpec, now time.Time) error {
	// One-shot schedules must have a start time in the future
	if spec.Cron == "" {
		if spec.Start.IsZero() {
			return fmt.Errorf("start time must be set for one-time schedules")
		}
		if spec.Start.Time.Before(now) {
			return fmt.Errorf("start time must be in the future for one-time schedules")
		}
		return nil
	}

	// Recurring schedules must have a positive duration
	if spec.Duration.Duration <= 0 {
		return fmt.Errorf("duration must be greater than 0 for recurring schedules")
	}

	// For recurring schedules, duration should be less than the minimum interval
	// This is a simplified check - in practice, you'd parse the cron to get the actual interval
	if spec.Duration.Duration >= 24*time.Hour {
		return fmt.Errorf("duration must be less than the minimum interval for recurring schedules")
	}

	return nil
}

func TestValidateScheduleSpec(t *testing.T) {
	now := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		spec    accessv1alpha1.ScheduleSpec
		wantErr bool
	}{
		{
			name: "oneshot start in past should fail",
			spec: accessv1alpha1.ScheduleSpec{
				Start: metav1.Time{Time: now.Add(-time.Hour)},
			},
			wantErr: true,
		},
		{
			name: "recurring requires positive duration",
			spec: accessv1alpha1.ScheduleSpec{
				Cron: "0 2 * * 6,0",
			},
			wantErr: true,
		},
		{
			name: "recurring duration must be less than interval",
			spec: accessv1alpha1.ScheduleSpec{
				Cron:     "0 2 * * 6,0",
				Duration: metav1.Duration{Duration: 7 * 24 * time.Hour},
				Start:    metav1.Time{Time: now},
			},
			wantErr: true,
		},
		{
			name: "recurring with future start is allowed",
			spec: accessv1alpha1.ScheduleSpec{
				Cron:     "0 2 * * 6,0",
				Start:    metav1.Time{Time: now.Add(2 * time.Hour)},
				Duration: metav1.Duration{Duration: 30 * time.Minute},
			},
			wantErr: false,
		},
		{
			name: "recurring with duration smaller than interval",
			spec: accessv1alpha1.ScheduleSpec{
				Cron:     "*/30 * * * *",
				Duration: metav1.Duration{Duration: 10 * time.Minute},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateScheduleSpec(&tt.spec, now)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
