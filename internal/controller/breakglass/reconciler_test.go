package breakglass

import (
	"testing"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/controller/breakglass/handlers"
	"github.com/stretchr/testify/assert"
)

type factoryType int

const (
	factoryPending factoryType = iota
	factoryApproved
	factoryRecurringActive
	factoryTerminal
)

func factoryKind(f func(*handlers.Handler) Controller) factoryType {
	h := &handlers.Handler{}
	c := f(h)
	switch c.(type) {
	case *handlers.PendingCondition:
		return factoryPending
	case *handlers.ApprovedCondition:
		return factoryApproved
	case *handlers.RecurringActiveCondition:
		return factoryRecurringActive
	case *handlers.TerminalCondition:
		return factoryTerminal
	default:
		return -1
	}
}

func TestHandlerFactoriesDispatch(t *testing.T) {
	tests := []struct {
		name        string
		lastCond    accessv1alpha1.BreakglassCondition
		expectKind  factoryType
		expectFound bool
	}{
		// “empty” keys
		{"NoCondition", accessv1alpha1.NoCondition, factoryPending, true},

		// explicit mappings
		{"Pending", accessv1alpha1.ConditionPending, factoryPending, true},
		{"Approved", accessv1alpha1.ConditionApproved, factoryApproved, true},
		{"Active", accessv1alpha1.ConditionActive, factoryRecurringActive, true},
		{"RecurringActive", accessv1alpha1.ConditionRecurringActive, factoryRecurringActive, true},

		// terminal states
		{"Denied", accessv1alpha1.ConditionDenied, factoryTerminal, true},
		{"Expired", accessv1alpha1.ConditionExpired, factoryTerminal, true},
		{"Revoked", accessv1alpha1.ConditionRevoked, factoryTerminal, true},

		// totally unknown → fallback to Pending
		{"FooBar", accessv1alpha1.BreakglassCondition("FooBar"), factoryPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, found := handlerFactories[tt.lastCond]
			if !found {
				// fallback factory
				factory = func(h *handlers.Handler) Controller {
					return handlers.NewPendingCondition(h)
				}
			}

			kind := factoryKind(factory)
			assert.Equal(t, tt.expectKind, kind, "factory kind mismatch for %q", tt.lastCond)
			assert.Equal(t, tt.expectFound, found, "found flag mismatch for %q", tt.lastCond)
		})
	}
}
