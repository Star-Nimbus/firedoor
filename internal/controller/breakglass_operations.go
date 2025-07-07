package controller

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"
	"github.com/cloud-nimbus/firedoor/internal/alerting"
	"github.com/cloud-nimbus/firedoor/internal/conditions"
	"github.com/cloud-nimbus/firedoor/internal/constants"
	"github.com/cloud-nimbus/firedoor/internal/telemetry"
)

// BreakglassOperator defines the interface for breakglass operations.
type BreakglassOperator interface {
	GrantAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error)
	RevokeAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error)
}

// breakglassOperator implements BreakglassOperator.
type breakglassOperator struct {
	client              client.Client
	recorder            record.EventRecorder
	alertService        *alerting.AlertmanagerService
	privilegeEscalation bool
}

// NewBreakglassOperator creates a new breakglass operator.
func NewBreakglassOperator(client client.Client, recorder record.EventRecorder, alertService *alerting.AlertmanagerService, privilegeEscalation bool) BreakglassOperator {
	return &breakglassOperator{
		client:              client,
		recorder:            recorder,
		alertService:        alertService,
		privilegeEscalation: privilegeEscalation,
	}
}

// errNoRequeue is a sentinel error type for permanent failures that should not be requeued
var errNoRequeue = fmt.Errorf("permanent failure - do not requeue")

// upsert creates or updates an object using controllerutil.CreateOrUpdate
func (o *breakglassOperator) upsert(ctx context.Context, obj client.Object, mutate func() error) error {
	_, err := controllerutil.CreateOrUpdate(ctx, o.client, obj, mutate)
	return err
}

// isPrivilegeEscalationEnabled returns true if privilege escalation mode is enabled
func (o *breakglassOperator) isPrivilegeEscalationEnabled() bool {
	return o.privilegeEscalation
}

// deduplicateSubjects removes duplicate subjects from the list
func deduplicateSubjects(subjects []rbacv1.Subject) []rbacv1.Subject {
	sbKey := func(s rbacv1.Subject) string {
		return s.Kind + "/" + s.Name + "/" + s.Namespace
	}
	set := make(map[string]rbacv1.Subject)
	for _, s := range subjects {
		set[sbKey(s)] = s
	}

	unique := make([]rbacv1.Subject, 0, len(set))
	for _, s := range set {
		unique = append(unique, s)
	}
	return unique
}

// safeName creates a safe resource name that won't exceed DNS-1123 limits
func safeName(parts ...string) string {
	// Simple implementation - in production, use k8s.io/apimachinery/pkg/util/validation/field
	name := "breakglass"
	for _, part := range parts {
		if len(name)+len(part)+1 <= 253 {
			name += "-" + part
		} else {
			break
		}
	}
	return name
}

// patchStatus updates the status with retry logic for conflicts
func (o *breakglassOperator) patchStatus(ctx context.Context, bg *accessv1alpha1.Breakglass, mutate func(*accessv1alpha1.Breakglass)) error {
	return retry.OnError(retry.DefaultRetry, apierrors.IsConflict, func() error {
		// Always refetch to avoid stale object conflicts
		latest := &accessv1alpha1.Breakglass{}
		if err := o.client.Get(ctx, client.ObjectKeyFromObject(bg), latest); err != nil {
			return err
		}
		// Apply mutations to the latest object
		mutate(latest)
		// Update the latest object
		return o.client.Status().Update(ctx, latest)
	})
}

// GrantAccess grants access to a breakglass resource.
func (o *breakglassOperator) GrantAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	ctx, span := telemetry.RecordGrantAccessStart(ctx, bg)
	defer span.End()

	// Request-scoped logger - capture once and thread it
	lg := log.FromContext(ctx).WithValues("breakglass", bg.Name)
	ctx = log.IntoContext(ctx, lg)

	subjects, err := o.resolveSubjectsAndHandleError(ctx, bg)
	if err != nil {
		if err == errNoRequeue {
			// Permanent failure, status has been set to Denied, do not requeue
			return ctrl.Result{}, nil
		}
		// Handle forbidden/invalid errors as permanent
		if apierrors.IsForbidden(err) || apierrors.IsInvalid(err) {
			lg.Error(err, "Permanent failure - do not requeue")
			return ctrl.Result{}, nil
		}
		// Other errors should be requeued
		return ctrl.Result{}, err
	}

	// Deduplicate subjects
	subjects = deduplicateSubjects(subjects)

	// Create RBAC resources based on the breakglass spec
	if err := o.createRBACResources(ctx, bg, subjects); err != nil {
		return ctrl.Result{}, err
	}

	return o.completeGrantAccess(ctx, bg, subjects)
}

// resolveSubjectsAndHandleError resolves the subjects and handles validation errors
// If validation fails, it sets status to Denied, emits an event, and returns errNoRequeue.
func (o *breakglassOperator) resolveSubjectsAndHandleError(ctx context.Context, bg *accessv1alpha1.Breakglass) ([]rbacv1.Subject, error) {
	subjects, err := resolveSubjects(ctx, bg)
	if err != nil {
		logger := log.FromContext(ctx)
		logger.Info("Breakglass missing subjects; denying access", "error", err)

		telemetry.RecordGrantAccessValidationFailure(bg)
		o.setDeniedStatus(ctx, bg, err)
		o.emitEvent(bg, corev1.EventTypeWarning, "InvalidRequest", "Missing subjects")

		// Return errNoRequeue to indicate this is a permanent failure and should not be requeued
		return nil, errNoRequeue
	}
	return subjects, nil
}

// createRBACResources creates the necessary RBAC resources (Roles/ClusterRoles and RoleBindings/ClusterRoleBindings)
func (o *breakglassOperator) createRBACResources(ctx context.Context, bg *accessv1alpha1.Breakglass, subjects []rbacv1.Subject) error {
	// Handle ClusterRoles if specified
	if len(bg.Spec.ClusterRoles) > 0 {
		return o.createClusterRoleBindings(ctx, bg, subjects)
	}

	// Handle AccessPolicy if specified
	if bg.Spec.AccessPolicy != nil && len(bg.Spec.AccessPolicy.Rules) > 0 {
		return o.createCustomRBACResources(ctx, bg, subjects)
	}

	return fmt.Errorf("neither clusterRoles nor accessPolicy specified")
}

// checkAndHandlePrivilegeEscalation checks if privilege escalation is needed and handles it appropriately
func (o *breakglassOperator) checkAndHandlePrivilegeEscalation(ctx context.Context, bg *accessv1alpha1.Breakglass, rules []rbacv1.PolicyRule) error {
	if !o.isPrivilegeEscalationEnabled() {
		// If privilege escalation is not enabled, we can't grant permissions we don't have
		// This is the default secure behavior
		return nil
	}

	// When privilege escalation is enabled, we can grant permissions we don't hold ourselves
	// This is handled by the RBAC escalate verb that should be granted to the operator
	log.FromContext(ctx).Info("privilege escalation enabled - operator can grant elevated permissions",
		"breakglass", bg.Name, "rules", len(rules))
	return nil
}

// ownerReferenceForBreakglass returns an OwnerReference for the given Breakglass resource.
func ownerReferenceForBreakglass(bg *accessv1alpha1.Breakglass, blockOwnerDeletion bool) metav1.OwnerReference {
	controller := true
	return metav1.OwnerReference{
		APIVersion:         bg.APIVersion,
		Kind:               bg.Kind,
		Name:               bg.Name,
		UID:                bg.UID,
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
}

// createClusterRoleBindings creates ClusterRoleBindings for existing ClusterRoles
func (o *breakglassOperator) createClusterRoleBindings(ctx context.Context, bg *accessv1alpha1.Breakglass, subjects []rbacv1.Subject) error {
	for _, clusterRoleName := range bg.Spec.ClusterRoles {
		// If AccessPolicy is present, create a custom ClusterRole with the given name and rules
		if bg.Spec.AccessPolicy != nil && len(bg.Spec.AccessPolicy.Rules) > 0 {
			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterRoleName,
					Labels: map[string]string{
						"breakglass": bg.Name,
						"managed-by": "firedoor",
					},
					OwnerReferences: []metav1.OwnerReference{ownerReferenceForBreakglass(bg, false)}, // Don't block deletion for cluster resources
				},
			}

			if err := o.upsert(ctx, clusterRole, func() error {
				rules := o.convertAccessRulesToPolicyRules(bg.Spec.AccessPolicy.Rules)

				// Check privilege escalation before setting rules
				if err := o.checkAndHandlePrivilegeEscalation(ctx, bg, rules); err != nil {
					return fmt.Errorf("privilege escalation check failed: %w", err)
				}

				clusterRole.Rules = rules
				return nil
			}); err != nil {
				telemetry.RecordGrantAccessFailure(bg, "cluster_role_upsert_failed")
				return fmt.Errorf("failed to upsert ClusterRole: %w", err)
			}
		}

		clusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: safeName(bg.Name, clusterRoleName),
				Labels: map[string]string{
					"breakglass": bg.Name,
					"managed-by": "firedoor",
				},
				OwnerReferences: []metav1.OwnerReference{ownerReferenceForBreakglass(bg, false)}, // Don't block deletion for cluster resources
			},
			Subjects: subjects,
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     clusterRoleName,
				APIGroup: rbacv1.GroupName,
			},
		}

		if err := o.upsert(ctx, clusterRoleBinding, func() error {
			clusterRoleBinding.Subjects = subjects
			return nil
		}); err != nil {
			telemetry.RecordGrantAccessFailure(bg, "cluster_role_binding_upsert_failed")
			return fmt.Errorf("failed to upsert ClusterRoleBinding: %w", err)
		}

		// Record role binding operation success
		telemetry.RecordOperation(telemetry.OpCreate, telemetry.ResultSuccess, telemetry.ComponentController, string(telemetry.RoleTypeClusterRole), fmt.Sprintf("cluster-wide-%s", clusterRoleName))
	}

	return nil
}

// createCustomRBACResources creates custom Roles/ClusterRoles and RoleBindings/ClusterRoleBindings based on AccessPolicy
func (o *breakglassOperator) createCustomRBACResources(ctx context.Context, bg *accessv1alpha1.Breakglass, subjects []rbacv1.Subject) error {
	// Determine if we need cluster-wide or namespace-specific resources
	isClusterWide := o.isClusterWideAccess(bg.Spec.AccessPolicy.Rules)

	if isClusterWide {
		return o.createClusterWideRBAC(ctx, bg, subjects)
	} else {
		return o.createNamespaceSpecificRBAC(ctx, bg, subjects)
	}
}

// isClusterWideAccess determines if the access policy requires cluster-wide permissions
func (o *breakglassOperator) isClusterWideAccess(rules []accessv1alpha1.AccessRule) bool {
	for _, rule := range rules {
		// If any rule has no namespaces specified, it's cluster-wide
		if len(rule.Namespaces) == 0 {
			return true
		}
	}
	return false
}

// createClusterWideRBAC creates ClusterRole and ClusterRoleBinding
func (o *breakglassOperator) createClusterWideRBAC(ctx context.Context, bg *accessv1alpha1.Breakglass, subjects []rbacv1.Subject) error {
	// Create ClusterRole
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: safeName(bg.Name),
			Labels: map[string]string{
				"breakglass": bg.Name,
				"managed-by": "firedoor",
			},
			OwnerReferences: []metav1.OwnerReference{ownerReferenceForBreakglass(bg, false)}, // Don't block deletion for cluster resources
		},
	}

	if err := o.upsert(ctx, clusterRole, func() error {
		rules := o.convertAccessRulesToPolicyRules(bg.Spec.AccessPolicy.Rules)

		// Check privilege escalation before setting rules
		if err := o.checkAndHandlePrivilegeEscalation(ctx, bg, rules); err != nil {
			return fmt.Errorf("privilege escalation check failed: %w", err)
		}

		clusterRole.Rules = rules
		return nil
	}); err != nil {
		return fmt.Errorf("failed to upsert ClusterRole: %w", err)
	}

	// Create ClusterRoleBinding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: safeName(bg.Name),
			Labels: map[string]string{
				"breakglass": bg.Name,
				"managed-by": "firedoor",
			},
			OwnerReferences: []metav1.OwnerReference{ownerReferenceForBreakglass(bg, false)}, // Don't block deletion for cluster resources
		},
		Subjects: subjects,
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
			APIGroup: rbacv1.GroupName,
		},
	}

	if err := o.upsert(ctx, clusterRoleBinding, func() error {
		clusterRoleBinding.Subjects = subjects
		return nil
	}); err != nil {
		return fmt.Errorf("failed to upsert ClusterRoleBinding: %w", err)
	}

	// Record role binding operation success
	telemetry.RecordOperation(telemetry.OpCreate, telemetry.ResultSuccess, telemetry.ComponentController, string(telemetry.RoleTypeClusterRole), "cluster-wide")

	return nil
}

// createNamespaceSpecificRBAC creates Role and RoleBinding for each namespace
// Uses child spans per namespace for granular timing and error tracking
func (o *breakglassOperator) createNamespaceSpecificRBAC(ctx context.Context, bg *accessv1alpha1.Breakglass, subjects []rbacv1.Subject) error {
	// Group rules by namespace
	namespaceRules := make(map[string][]rbacv1.PolicyRule)

	for _, rule := range bg.Spec.AccessPolicy.Rules {
		// Validate that namespace-specific rules have namespaces
		if len(rule.Namespaces) == 0 {
			return fmt.Errorf("rule without namespace in namespace-specific block")
		}

		for _, namespace := range rule.Namespaces {
			policyRule := o.convertAccessRuleToPolicyRule(rule)
			namespaceRules[namespace] = append(namespaceRules[namespace], policyRule)
		}
	}

	// Create Role and RoleBinding for each namespace with child spans
	for namespace, rules := range namespaceRules {
		err := telemetry.RecordNamespaceOperation(ctx, "CreateNamespaceRBAC", namespace, func() error {
			// Create Role
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      safeName(bg.Name),
					Namespace: namespace,
					Labels: map[string]string{
						"breakglass":           bg.Name,
						"managed-by":           "firedoor",
						"breakglass-namespace": bg.Namespace, // Track source namespace for cleanup
					},
				},
			}

			if err := o.upsert(ctx, role, func() error {
				// Check privilege escalation before setting rules
				if err := o.checkAndHandlePrivilegeEscalation(ctx, bg, rules); err != nil {
					return fmt.Errorf("privilege escalation check failed: %w", err)
				}

				role.Rules = rules
				// Set controller reference only for same-namespace resources
				if bg.Namespace == role.Namespace {
					if err := controllerutil.SetControllerReference(bg, role, o.client.Scheme()); err != nil {
						return fmt.Errorf("failed to set controller reference for Role: %w", err)
					}
				} else {
					log.FromContext(ctx).Info("skip ownerRef – cross-namespace", "bgNs", bg.Namespace, "roleNs", role.Namespace)
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to upsert Role in namespace %s: %w", namespace, err)
			}

			// Create RoleBinding
			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      safeName(bg.Name),
					Namespace: namespace,
					Labels: map[string]string{
						"breakglass":           bg.Name,
						"managed-by":           "firedoor",
						"breakglass-namespace": bg.Namespace, // Track source namespace for cleanup
					},
				},
				Subjects: subjects,
				RoleRef: rbacv1.RoleRef{
					Kind:     "Role",
					Name:     role.Name,
					APIGroup: rbacv1.GroupName,
				},
			}

			if err := o.upsert(ctx, roleBinding, func() error {
				roleBinding.Subjects = subjects
				// Set controller reference only for same-namespace resources
				if bg.Namespace == roleBinding.Namespace {
					if err := controllerutil.SetControllerReference(bg, roleBinding, o.client.Scheme()); err != nil {
						return fmt.Errorf("failed to set controller reference for RoleBinding: %w", err)
					}
				} else {
					log.FromContext(ctx).Info("skip ownerRef – cross-namespace", "bgNs", bg.Namespace, "roleBindingNs", roleBinding.Namespace)
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to upsert RoleBinding in namespace %s: %w", namespace, err)
			}

			// Record role binding operation success
			telemetry.RecordOperation(telemetry.OpCreate, telemetry.ResultSuccess, telemetry.ComponentController, string(telemetry.RoleTypeRole), fmt.Sprintf("namespace-%s", namespace))

			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// convertAccessRulesToPolicyRules converts AccessRules to RBAC PolicyRules
func (o *breakglassOperator) convertAccessRulesToPolicyRules(accessRules []accessv1alpha1.AccessRule) []rbacv1.PolicyRule {
	var policyRules []rbacv1.PolicyRule

	for _, accessRule := range accessRules {
		policyRule := o.convertAccessRuleToPolicyRule(accessRule)
		policyRules = append(policyRules, policyRule)
	}

	return policyRules
}

// convertAccessRuleToPolicyRule converts a single AccessRule to RBAC PolicyRule
func (o *breakglassOperator) convertAccessRuleToPolicyRule(accessRule accessv1alpha1.AccessRule) rbacv1.PolicyRule {
	// Convert Actions to Verbs
	var verbs []string
	for _, action := range accessRule.Actions {
		verbs = append(verbs, string(action))
	}

	return rbacv1.PolicyRule{
		Verbs:         verbs,
		APIGroups:     accessRule.APIGroups,
		Resources:     accessRule.Resources,
		ResourceNames: accessRule.ResourceNames,
	}
}

// setDeniedStatus sets the denied status and conditions
func (o *breakglassOperator) setDeniedStatus(ctx context.Context, bg *accessv1alpha1.Breakglass, err error) {
	// Collapse status updates into one patch
	if err := o.patchStatus(ctx, bg, func(latest *accessv1alpha1.Breakglass) {
		latest.Status.Phase = accessv1alpha1.PhaseDenied
		latest.Status.ApprovedBy = constants.ControllerIdentity

		o.createCondition(latest, conditions.Denied, metav1.ConditionTrue, conditions.InvalidRequest, fmt.Sprintf("Missing subjects: %v", err))
		o.createCondition(latest, conditions.Approved, metav1.ConditionFalse, conditions.InvalidRequest, "Request denied due to missing user or group")
	}); err != nil {
		telemetry.RecordStatusUpdateError("update")
	}
}

// completeGrantAccess completes the grant access process
func (o *breakglassOperator) completeGrantAccess(ctx context.Context, bg *accessv1alpha1.Breakglass, subjects []rbacv1.Subject) (ctrl.Result, error) {
	now := metav1.Now()

	// Calculate expiry time
	var expiry metav1.Time
	if bg.Spec.Duration != nil {
		expiry = metav1.NewTime(now.Time.Add(bg.Spec.Duration.Duration))
	} else {
		// Default to 1 hour if no duration specified
		expiry = metav1.NewTime(now.Time.Add(1 * time.Hour))
	}

	// Set approval info if not already set
	if bg.Status.ApprovedBy == "" {
		bg.Status.ApprovedBy = constants.ControllerIdentity
		bg.Status.ApprovedAt = &now
	}

	telemetry.RecordGrantAccessSuccess(bg, subjects[0].Name)

	// Determine the correct phase based on whether this is a recurring breakglass
	phase := accessv1alpha1.PhaseActive
	if bg.Spec.Recurring {
		phase = accessv1alpha1.PhaseRecurringActive
	}

	// For recurring breakglasses, preserve any existing NextActivationAt that might have been set
	// by ShouldActivateRecurring, but ensure we set the proper phase and timing
	if bg.Spec.Recurring {
		// Ensure we have the proper recurring status fields
		if bg.Status.NextActivationAt == nil {
			// This shouldn't happen if ShouldActivateRecurring was called, but handle it gracefully
			lg := log.FromContext(ctx)
			lg.Info("NextActivationAt was nil for recurring breakglass, setting default")
		}
	}

	// Collapse status updates into one patch
	if err := o.patchStatus(ctx, bg, func(latest *accessv1alpha1.Breakglass) {
		o.updateGrantedStatus(latest, phase, &now, &expiry)
		o.setGrantedConditions(latest, subjects, expiry)
	}); err != nil {
		telemetry.RecordStatusUpdateError("update")
		return ctrl.Result{}, err
	}

	// Send alert for active breakglass
	if o.alertService != nil {
		if err := o.alertService.SendBreakglassActiveAlert(ctx, bg); err != nil {
			log := log.FromContext(ctx)
			log.Error(err, "Failed to send breakglass active alert")
			// Don't fail the operation if alert sending fails
		} else {
			// Add span event for successful alert
			if span := trace.SpanFromContext(ctx); span != nil {
				span.AddEvent("alert.sent")
			}
		}
	}

	log := log.FromContext(ctx)
	log.Info("Granted breakglass access", "subjects", len(subjects), "expiresAt", expiry)

	// Avoid hot loops on negative time.Until
	d := time.Until(expiry.Time)
	if d < 5*time.Second {
		d = 5 * time.Second
	}
	return ctrl.Result{RequeueAfter: d}, nil
}

// updateGrantedStatus updates the status with granted information
func (o *breakglassOperator) updateGrantedStatus(bg *accessv1alpha1.Breakglass, phase accessv1alpha1.BreakglassPhase, now, expiry *metav1.Time) {
	bg.Status.Phase = phase
	bg.Status.GrantedAt = now
	bg.Status.ExpiresAt = expiry
}

// setGrantedConditions sets the conditions for granted access
func (o *breakglassOperator) setGrantedConditions(bg *accessv1alpha1.Breakglass, subjects []rbacv1.Subject, expiry metav1.Time) {
	o.createCondition(bg, conditions.Approved, metav1.ConditionTrue, conditions.AccessGranted, fmt.Sprintf("Breakglass access granted to %d subjects until %s", len(subjects), expiry.Format(time.RFC3339)))

	// Set appropriate active condition based on whether this is recurring
	if bg.Spec.Recurring {
		o.createCondition(bg, conditions.RecurringActive, metav1.ConditionTrue, conditions.RecurringAccessActive, "Recurring access is currently active")
	} else {
		o.createCondition(bg, conditions.Active, metav1.ConditionTrue, conditions.AccessActive, fmt.Sprintf("Breakglass access is active until %s", expiry.Format(time.RFC3339)))
	}

	o.createCondition(bg, conditions.Denied, metav1.ConditionFalse, conditions.AccessGranted, "Access has been granted")
}

// RevokeAccess revokes access from a breakglass resource.
func (o *breakglassOperator) RevokeAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	ctx, span := telemetry.RecordRevokeAccessStart(ctx, bg)
	defer span.End()

	// Request-scoped logger - capture once and thread it
	lg := log.FromContext(ctx).WithValues("breakglass", bg.Name)
	ctx = log.IntoContext(ctx, lg)

	// Prevent finalization race: if object is being deleted and already expired, skip
	if bg.DeletionTimestamp != nil && bg.Status.Phase == accessv1alpha1.PhaseExpired {
		lg.Info("Skipping revoke; already expired and being finalized")
		return ctrl.Result{}, nil
	}

	if err := o.deleteRBACResources(ctx, bg); err != nil {
		return ctrl.Result{}, err
	}

	return o.completeRevokeAccess(ctx, bg)
}

// deleteRBACResources deletes all RBAC resources created for this breakglass
func (o *breakglassOperator) deleteRBACResources(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	lg := log.FromContext(ctx).V(1)

	// Delete ClusterRoleBindings for ClusterRoles
	if len(bg.Spec.ClusterRoles) > 0 {
		for _, clusterRoleName := range bg.Spec.ClusterRoles {
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			clusterRoleBindingName := safeName(bg.Name, clusterRoleName)

			if err := o.client.Get(ctx, client.ObjectKey{Name: clusterRoleBindingName}, clusterRoleBinding); err == nil {
				lg.Info("Deleting ClusterRoleBinding", "name", clusterRoleBindingName)
				// Retry on conflict or server timeout
				if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
					return apierrors.IsConflict(err) || apierrors.IsServerTimeout(err)
				}, func() error {
					return o.client.Delete(ctx, clusterRoleBinding)
				}); err != nil && !apierrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete ClusterRoleBinding %s: %w", clusterRoleBindingName, err)
				}
			}
		}
		return nil
	}

	// Delete custom RBAC resources
	if bg.Spec.AccessPolicy != nil && len(bg.Spec.AccessPolicy.Rules) > 0 {
		isClusterWide := o.isClusterWideAccess(bg.Spec.AccessPolicy.Rules)

		if isClusterWide {
			return o.deleteClusterWideRBAC(ctx, bg)
		} else {
			return o.deleteNamespaceSpecificRBAC(ctx, bg)
		}
	}

	return nil
}

// deleteClusterWideRBAC deletes ClusterRole and ClusterRoleBinding
func (o *breakglassOperator) deleteClusterWideRBAC(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	lg := log.FromContext(ctx).V(1)

	// Delete ClusterRoleBinding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	clusterRoleBindingName := safeName(bg.Name)

	if err := o.client.Get(ctx, client.ObjectKey{Name: clusterRoleBindingName}, clusterRoleBinding); err == nil {
		lg.Info("Deleting ClusterRoleBinding", "name", clusterRoleBindingName)
		// Retry on conflict or server timeout
		if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
			return apierrors.IsConflict(err) || apierrors.IsServerTimeout(err)
		}, func() error {
			return o.client.Delete(ctx, clusterRoleBinding)
		}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ClusterRoleBinding: %w", err)
		}
	}

	// Delete ClusterRole
	clusterRole := &rbacv1.ClusterRole{}
	if err := o.client.Get(ctx, client.ObjectKey{Name: clusterRoleBindingName}, clusterRole); err == nil {
		lg.Info("Deleting ClusterRole", "name", clusterRoleBindingName)
		// Retry on conflict or server timeout
		if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
			return apierrors.IsConflict(err) || apierrors.IsServerTimeout(err)
		}, func() error {
			return o.client.Delete(ctx, clusterRole)
		}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ClusterRole: %w", err)
		}
	}

	return nil
}

// deleteNamespaceSpecificRBAC deletes Roles and RoleBindings from all namespaces
func (o *breakglassOperator) deleteNamespaceSpecificRBAC(ctx context.Context, bg *accessv1alpha1.Breakglass) error {
	lg := log.FromContext(ctx).V(1)

	// Get all namespaces where we created RBAC resources
	namespaces := make(map[string]bool)
	for _, rule := range bg.Spec.AccessPolicy.Rules {
		for _, namespace := range rule.Namespaces {
			namespaces[namespace] = true
		}
	}

	// Also check for resources with our labels in case of cross-namespace cleanup
	roleList := &rbacv1.RoleList{}
	if err := o.client.List(ctx, roleList, client.MatchingLabels(map[string]string{
		"breakglass": bg.Name,
		"managed-by": "firedoor",
	})); err == nil {
		for _, role := range roleList.Items {
			namespaces[role.Namespace] = true
		}
	}

	roleBindingList := &rbacv1.RoleBindingList{}
	if err := o.client.List(ctx, roleBindingList, client.MatchingLabels(map[string]string{
		"breakglass": bg.Name,
		"managed-by": "firedoor",
	})); err == nil {
		for _, roleBinding := range roleBindingList.Items {
			namespaces[roleBinding.Namespace] = true
		}
	}

	// Delete Role and RoleBinding from each namespace
	for namespace := range namespaces {
		roleName := safeName(bg.Name)

		// Delete RoleBinding
		roleBinding := &rbacv1.RoleBinding{}
		if err := o.client.Get(ctx, client.ObjectKey{Name: roleName, Namespace: namespace}, roleBinding); err == nil {
			lg.Info("Deleting RoleBinding", "name", roleName, "namespace", namespace)
			// Retry on conflict or server timeout
			if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
				return apierrors.IsConflict(err) || apierrors.IsServerTimeout(err)
			}, func() error {
				return o.client.Delete(ctx, roleBinding)
			}); err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete RoleBinding in namespace %s: %w", namespace, err)
			}
		}

		// Delete Role
		role := &rbacv1.Role{}
		if err := o.client.Get(ctx, client.ObjectKey{Name: roleName, Namespace: namespace}, role); err == nil {
			lg.Info("Deleting Role", "name", roleName, "namespace", namespace)
			// Retry on conflict or server timeout
			if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
				return apierrors.IsConflict(err) || apierrors.IsServerTimeout(err)
			}, func() error {
				return o.client.Delete(ctx, role)
			}); err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete Role in namespace %s: %w", namespace, err)
			}
		}
	}

	return nil
}

// completeRevokeAccess completes the revoke access process
func (o *breakglassOperator) completeRevokeAccess(ctx context.Context, bg *accessv1alpha1.Breakglass) (ctrl.Result, error) {
	telemetry.RecordRevokeAccessSuccess(bg)

	// Collapse status updates into one patch
	if err := o.patchStatus(ctx, bg, func(latest *accessv1alpha1.Breakglass) {
		o.updateRevokedStatus(latest)
		o.setRevokedConditions(latest)
	}); err != nil {
		telemetry.RecordStatusUpdateError("update")
		return ctrl.Result{}, err
	}

	// Send alert for expired breakglass
	if o.alertService != nil {
		if err := o.alertService.SendBreakglassExpiredAlert(ctx, bg); err != nil {
			log := log.FromContext(ctx)
			log.Error(err, "Failed to send breakglass expired alert")
			// Don't fail the operation if alert sending fails
		} else {
			// Add span event for successful alert
			if span := trace.SpanFromContext(ctx); span != nil {
				span.AddEvent("alert.sent")
			}
		}
	}

	log := log.FromContext(ctx)
	log.Info("Revoked breakglass access")
	return ctrl.Result{}, nil
}

// updateRevokedStatus updates the status for revoked access
func (o *breakglassOperator) updateRevokedStatus(bg *accessv1alpha1.Breakglass) {
	bg.Status.Phase = accessv1alpha1.PhaseExpired
}

// setRevokedConditions sets the conditions for revoked access
func (o *breakglassOperator) setRevokedConditions(bg *accessv1alpha1.Breakglass) {
	o.createCondition(bg, conditions.Expired, metav1.ConditionTrue, conditions.AccessExpired, "Breakglass access expired and revoked")
	o.createCondition(bg, conditions.Active, metav1.ConditionFalse, conditions.AccessExpired, "Access is no longer active")
}

// emitEvent emits a Kubernetes event
func (o *breakglassOperator) emitEvent(bg *accessv1alpha1.Breakglass, eventType, reason, message string) {
	o.recorder.Event(bg, eventType, reason, message)
}

// createCondition creates a new condition using meta.SetStatusCondition to avoid duplicates
func (o *breakglassOperator) createCondition(bg *accessv1alpha1.Breakglass, conditionType conditions.Condition, status metav1.ConditionStatus, reason conditions.Reason, message string) {
	condition := metav1.Condition{
		Type:               string(conditionType),
		Status:             status,
		Reason:             string(reason),
		Message:            message,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: bg.Generation,
	}
	meta.SetStatusCondition(&bg.Status.Conditions, condition)
}
