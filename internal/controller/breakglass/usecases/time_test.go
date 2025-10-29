package usecases

import (
	"testing"
	"time"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCurrentWindowRecurringInWindow(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 15, 0, 0, time.UTC)
	bg := &accessv1alpha1.Breakglass{
		Spec: accessv1alpha1.BreakglassSpec{
			Schedule: accessv1alpha1.ScheduleSpec{
				Cron:     "0 * * * *",
				Duration: metav1.Duration{Duration: 30 * time.Minute},
			},
		},
	}

	window, ok := CurrentWindow(bg, now)
	if !ok {
		t.Fatalf("expected window, got none")
	}

	wantStart := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	wantEnd := wantStart.Add(30 * time.Minute)

	if !window.Start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", window.Start, wantStart)
	}
	if !window.End.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", window.End, wantEnd)
	}
}

func TestCurrentWindowRecurringBeforeStart(t *testing.T) {
	now := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	start := metav1.NewTime(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	bg := &accessv1alpha1.Breakglass{
		Spec: accessv1alpha1.BreakglassSpec{
			Schedule: accessv1alpha1.ScheduleSpec{
				Cron:     "0 * * * *",
				Duration: metav1.Duration{Duration: 15 * time.Minute},
				Start:    start,
			},
		},
		Status: accessv1alpha1.BreakglassStatus{
			NextActivationAt: &start,
		},
	}

	if _, ok := CurrentWindow(bg, now); ok {
		t.Fatalf("expected no window before start")
	}
}

func TestCurrentWindowOneShot(t *testing.T) {
	start := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	now := start.Add(45 * time.Minute)
	bg := &accessv1alpha1.Breakglass{
		Spec: accessv1alpha1.BreakglassSpec{
			Schedule: accessv1alpha1.ScheduleSpec{
				Duration: metav1.Duration{Duration: time.Hour},
				Start:    metav1.NewTime(start),
			},
		},
	}

	window, ok := CurrentWindow(bg, now)
	if !ok {
		t.Fatalf("expected window, got none")
	}

	if !window.Start.Equal(start) {
		t.Errorf("start = %v, want %v", window.Start, start)
	}

	wantEnd := start.Add(time.Hour)
	if !window.End.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", window.End, wantEnd)
	}
}

func TestFinalCompletionTime(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC)
	bg := &accessv1alpha1.Breakglass{
		Spec: accessv1alpha1.BreakglassSpec{
			Schedule: accessv1alpha1.ScheduleSpec{
				Cron:     "*/15 * * * *",
				Duration: metav1.Duration{Duration: 20 * time.Minute},
			},
		},
	}

	got, ok := FinalCompletionTime(bg, now)
	if !ok {
		t.Fatalf("expected completion time")
	}

	wantEnd := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC).Add(20 * time.Minute)
	if !got.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", got, wantEnd)
	}
}
