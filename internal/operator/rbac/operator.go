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

package rbac

import (
	"context"
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/errors"

	cronv3 "github.com/robfig/cron/v3"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Operator implements the controller.BreakglassOperator interface.
type Operator struct {
	client client.Client
}

// Compile-time assertion: ensure Operator implements controller.BreakglassOperator
var _ interface {
	GrantAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) error
	RevokeAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) error
	ValidateAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) error
} = (*Operator)(nil)

func New(c client.Client) *Operator {
	return &Operator{client: c}
}

// GrantAccess creates the necessary RBAC resources for a breakglass request
func (o *Operator) GrantAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	log := ctrl.LoggerFrom(ctx)
	uidSuffix := string(bg.UID)[:8] // Use first 8 chars of UID for brevity
	labels := o.getBreakglassLabels(bg)
	createdResources := make([]string, 0)

	// Grant ClusterRoles
	clusterRoleResources, err := o.createClusterRoleBindings(ctx, bg, uidSuffix, labels)
	if err != nil {
		return err
	}
	createdResources = append(createdResources, clusterRoleResources...)

	// Grant ad-hoc Policy rules as Roles/RoleBindings
	policyResources, err := o.createPolicyResources(ctx, bg, uidSuffix, labels)
	if err != nil {
		return err
	}
	createdResources = append(createdResources, policyResources...)

	// Update status with created resources
	if len(createdResources) > 0 {
		log.Info("created breakglass RBAC resources", "resources", createdResources)
		return o.updateStatusWithCreatedResources(ctx, bg, createdResources)
	}

	return nil
}

// getBreakglassLabels returns the standard labels for breakglass resources
func (o *Operator) getBreakglassLabels(bg *accessv1alpha1.Breakglass) map[string]string {
	return map[string]string{
		"breakglass/name":      bg.Name,
		"breakglass/namespace": bg.Namespace,
		"breakglass/uid":       string(bg.UID),
		"breakglass-uid":       string(bg.UID), // Additional unique label
	}
}

// createClusterRoleBindings creates ClusterRoleBindings for the specified cluster roles
func (o *Operator) createClusterRoleBindings(
	ctx context.Context,
	bg *accessv1alpha1.Breakglass,
	uidSuffix string,
	labels map[string]string,
) ([]string, error) {
	createdResources := make([]string, 0)
	log := ctrl.LoggerFrom(ctx)

	for _, cr := range bg.Spec.ClusterRoles {
		crbName := fmt.Sprintf("breakglass-%s-clusterrolebinding-%s", uidSuffix, cr)

		// Skip if already created
		if contains(bg.Status.CreatedResources, crbName) {
			continue
		}

		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   crbName,
				Labels: labels,
			},
			Subjects: bg.Spec.Subjects,
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     cr,
			},
		}

		if err := o.createResourceWithTimeout(ctx, crb, "ClusterRoleBinding "+crbName); err != nil {
			return nil, err
		}
		createdResources = append(createdResources, crbName)
		log.Info("created clusterrolebinding", "name", crbName)
	}

	return createdResources, nil
}

// createPolicyResources creates Roles and RoleBindings for the specified policies
func (o *Operator) createPolicyResources(
	ctx context.Context,
	bg *accessv1alpha1.Breakglass,
	uidSuffix string,
	labels map[string]string,
) ([]string, error) {
	createdResources := make([]string, 0)
	log := ctrl.LoggerFrom(ctx)

	for i, policy := range bg.Spec.Policy {
		roleName := fmt.Sprintf("breakglass-%s-role-%d", uidSuffix, i)
		rbName := fmt.Sprintf("breakglass-%s-rolebinding-%d", uidSuffix, i)

		// Skip if already created
		if contains(bg.Status.CreatedResources, roleName) && contains(bg.Status.CreatedResources, rbName) {
			continue
		}

		// Create Role
		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleName,
				Namespace: policy.Namespace,
				Labels:    labels,
			},
			Rules: policy.Rules,
		}
		if err := o.createResourceWithTimeout(
			ctx,
			role,
			fmt.Sprintf("Role %s in %s", roleName, policy.Namespace),
		); err != nil {
			return nil, err
		}
		createdResources = append(createdResources, roleName)
		log.Info("created role", "namespace", policy.Namespace, "name", roleName)

		// Create RoleBinding
		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rbName,
				Namespace: policy.Namespace,
				Labels:    labels,
			},
			Subjects: bg.Spec.Subjects,
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     roleName,
			},
		}
		if err := o.createResourceWithTimeout(
			ctx,
			rb,
			fmt.Sprintf("RoleBinding %s in %s", rbName, policy.Namespace),
		); err != nil {
			return nil, err
		}
		createdResources = append(createdResources, rbName)
		log.Info("created rolebinding", "namespace", policy.Namespace, "name", rbName)
	}

	return createdResources, nil
}

// createResourceWithTimeout creates a resource with a 30-second timeout and proper error handling
func (o *Operator) createResourceWithTimeout(ctx context.Context, obj client.Object, resourceDesc string) error {
	childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := o.client.Create(childCtx, obj); err != nil {
		if errors.IsAlreadyExistsError(err) {
			// Resource already exists, this is not an error
			return nil
		}
		if errors.IsRetryableK8sError(err) {
			return errors.NewRetryableRBACError("creating", resourceDesc, accessv1alpha1.ReasonRBACTimeout, err)
		}
		return errors.NewPermanentRBACError("creating", resourceDesc, accessv1alpha1.ReasonRBACForbidden, err)
	}
	return nil
}

// updateStatusWithCreatedResources updates the breakglass status with newly created resources
func (o *Operator) updateStatusWithCreatedResources(
	ctx context.Context,
	bg *accessv1alpha1.Breakglass,
	createdResources []string,
) error {
	bg.Status.CreatedResources = append(bg.Status.CreatedResources, createdResources...)
	childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := o.client.Status().Update(childCtx, bg); err != nil {
		if errors.IsRetryableK8sError(err) {
			return errors.NewRetryableRBACError("updating", "Breakglass status", accessv1alpha1.ReasonRBACTimeout, err)
		}
		return errors.NewPermanentRBACError("updating", "Breakglass status", accessv1alpha1.ReasonRBACForbidden, err)
	}
	return nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RevokeAccess removes the RBAC resources for a breakglass request
func (o *Operator) RevokeAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	// Just call CleanupResources for now (idempotent)
	return o.CleanupResources(ctx, bg)
}

// ValidateAccess validates if the breakglass request can be created
func (o *Operator) ValidateAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	// Validate recurring schedule if cron is set
	if bg.Spec.Schedule.Cron != "" {
		// Parse the cron expression to validate it
		parser := cronv3.NewParser(cronv3.Minute | cronv3.Hour | cronv3.Dom | cronv3.Month | cronv3.Dow)
		if _, err := parser.Parse(bg.Spec.Schedule.Cron); err != nil {
			return fmt.Errorf("invalid cron schedule '%s': %w", bg.Spec.Schedule.Cron, err)
		}
	}
	return nil
}

// CleanupResources deletes all RBAC resources associated with the breakglass request
func (o *Operator) CleanupResources(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	labels := o.getBreakglassLabels(bg)

	// Delete resources in order: RoleBindings, Roles, ClusterRoleBindings
	if err := o.deleteRoleBindings(ctx, labels); err != nil {
		return err
	}

	if err := o.deleteRoles(ctx, labels); err != nil {
		return err
	}

	if err := o.deleteClusterRoleBindings(ctx, labels); err != nil {
		return err
	}

	return nil
}

// deleteRoleBindings deletes all RoleBindings with the specified labels
func (o *Operator) deleteRoleBindings(ctx context.Context, labels map[string]string) error {
	var rbList rbacv1.RoleBindingList
	if err := o.listResourcesWithTimeout(ctx, &rbList, labels); err != nil {
		return errors.NewRetryableRBACError("listing", "RoleBindings", accessv1alpha1.ReasonRBACTimeout, err)
	}

	log := ctrl.LoggerFrom(ctx)
	for i := range rbList.Items {
		rb := &rbList.Items[i]
		if err := o.deleteResourceWithTimeout(
			ctx,
			rb,
			fmt.Sprintf("RoleBinding %s/%s", rb.Namespace, rb.Name),
		); err != nil {
			return err
		}
		log.Info("deleted rolebinding", "namespace", rb.Namespace, "name", rb.Name)
	}
	return nil
}

// deleteRoles deletes all Roles with the specified labels
func (o *Operator) deleteRoles(ctx context.Context, labels map[string]string) error {
	var roleList rbacv1.RoleList
	if err := o.listResourcesWithTimeout(ctx, &roleList, labels); err != nil {
		return errors.NewRetryableRBACError("listing", "Roles", accessv1alpha1.ReasonRBACTimeout, err)
	}

	log := ctrl.LoggerFrom(ctx)
	for i := range roleList.Items {
		role := &roleList.Items[i]
		if err := o.deleteResourceWithTimeout(
			ctx,
			role,
			fmt.Sprintf("Role %s/%s", role.Namespace, role.Name),
		); err != nil {
			return err
		}
		log.Info("deleted role", "namespace", role.Namespace, "name", role.Name)
	}
	return nil
}

// deleteClusterRoleBindings deletes all ClusterRoleBindings with the specified labels
func (o *Operator) deleteClusterRoleBindings(ctx context.Context, labels map[string]string) error {
	var crbList rbacv1.ClusterRoleBindingList
	if err := o.listResourcesWithTimeout(ctx, &crbList, labels); err != nil {
		return errors.NewRetryableRBACError("listing", "ClusterRoleBindings", accessv1alpha1.ReasonRBACTimeout, err)
	}

	log := ctrl.LoggerFrom(ctx)
	for i := range crbList.Items {
		crb := &crbList.Items[i]
		if err := o.deleteResourceWithTimeout(
			ctx,
			crb,
			"ClusterRoleBinding "+crb.Name,
		); err != nil {
			return err
		}
		log.Info("deleted clusterrolebinding", "name", crb.Name)
	}
	return nil
}

// listResourcesWithTimeout lists resources with a 30-second timeout
func (o *Operator) listResourcesWithTimeout(
	ctx context.Context,
	list client.ObjectList,
	labels map[string]string,
) error {
	childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	listOpts := []client.ListOption{client.MatchingLabels(labels)}
	return o.client.List(childCtx, list, listOpts...)
}

// deleteResourceWithTimeout deletes a resource with a 30-second timeout and proper error handling
func (o *Operator) deleteResourceWithTimeout(
	ctx context.Context,
	obj client.Object,
	resourceDesc string,
) error {
	childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := o.client.Delete(childCtx, obj)
	if ign := client.IgnoreNotFound(err); ign != nil {
		if errors.IsRetryableK8sError(ign) {
			return errors.NewRetryableRBACError("deleting", resourceDesc, accessv1alpha1.ReasonRBACTimeout, ign)
		}
		return errors.NewPermanentRBACError("deleting", resourceDesc, accessv1alpha1.ReasonRBACForbidden, ign)
	}
	return nil
}
