package recurring

import (
	"context"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
)

type NoopManager struct{}

func (n NoopManager) ProcessRecurring(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	return nil
}
func (n NoopManager) ShouldActivate(ctx context.Context, bg *accessv1alpha1.Breakglass) bool {
	return false
}
func (n NoopManager) ShouldDeactivate(ctx context.Context, bg *accessv1alpha1.Breakglass) bool {
	return false
}
func (n NoopManager) OnActivationGranted(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	return nil
}
