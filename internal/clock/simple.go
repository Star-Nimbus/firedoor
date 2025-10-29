package clock

import (
	"time"

	"github.com/cloud-nimbus/firedoor/internal/controller"
)

// New creates a new Clock instance.
func New() controller.Clock {
	return SimpleClock{}
}

// SimpleClock is the default Clock.
type SimpleClock struct{}

func (SimpleClock) Now() time.Time { return time.Now() }

// Until returns the duration until t, or 0 if t is zero or less than.
func (SimpleClock) Until(t time.Time) time.Duration {
	if t.IsZero() {
		return -1 // sentil:
	}
	if time.Until(t) < 0 {
		return 0
	}
	return time.Until(t)
}

// Is Expired returns true if the given time is in the past or false if t is zero (no expiry).
func (SimpleClock) IsExpired(t time.Time) bool {
	if t.IsZero() {
		return false // no expiry
	}
	return time.Now().After(t)
}
