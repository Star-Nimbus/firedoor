package rbac

import (
	"context"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
)

type NoopOperator struct{}

func (n NoopOperator) GrantAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	return nil
}
func (n NoopOperator) RevokeAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	return nil
}
func (n NoopOperator) ValidateAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	return nil
}
