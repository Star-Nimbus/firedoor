package clock

import (
	"testing"
	"time"
)

func TestSimpleClock_IsExpired(t *testing.T) {
	c := SimpleClock{}

	now := time.Now().UTC()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)
	zero := time.Time{}

	tests := []struct {
		name  string
		want  bool
		input time.Time
	}{
		{
			name:  "past time should expire",
			input: past,
			want:  true,
		},
		{
			name:  "future time should not expire",
			input: future,
			want:  false,
		},
		{
			name:  "zero time should not expire",
			input: zero,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.IsExpired(tt.input)
			if got != tt.want {
				t.Errorf("%s: IsExpired(%v), got = %v, want = %v", tt.name, tt.input, got, tt.want)
			}
		})
	}
}

func TestSimpleClock_Until(t *testing.T) {
	c := SimpleClock{}

	now := time.Now().UTC()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)
	zero := time.Time{}

	tests := []struct {
		name  string
		want  time.Duration
		input time.Time
	}{
		{
			name:  "past time should zero",
			input: past,
			want:  0,
		},
		{
			name:  "future time should return an hour",
			input: future,
			want:  time.Hour,
		},
		{
			name:  "zero time should return 0",
			input: zero,
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Until(tt.input)
			if got.Round(time.Second) != tt.want.Round(time.Second) {
				t.Errorf("%s: IsExpired(%v), got = %v, want = %v", tt.name, tt.input, got, tt.want)
			}
		})
	}
}
