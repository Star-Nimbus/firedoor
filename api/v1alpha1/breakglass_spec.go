package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BreakglassSpec defines the desired state of a Breakglass access request.
type BreakglassSpec struct {
	// Subjects defines the users/groups/service accounts to grant temporary access.
	// At least one subject is required.
	// +kubebuilder:validation:MinItems=1
	Subjects []rbacv1.Subject `json:"subjects"`

	// Either specify an ad-hoc policy or reuse existing ClusterRoles by name.
	// Exactly one must be set.
	// +kubebuilder:validation:XOR=policy;clusterRoles
	Policy       []Policy `json:"policy,omitempty"`
	ClusterRoles []string `json:"clusterRoles,omitempty"`

	// Approval requirements.
	// +optional
	Approval *ApprovalSpec `json:"approval,omitempty"`

	// Schedule defines the breakglass activation window with optional cron recurrence.
	Schedule ScheduleSpec `json:"schedule"`

	// A clear, human-readable justification is required.
	// +kubebuilder:validation:MinLength=1
	Justification string `json:"justification"`

	// Optional external ticket identifier.
	// +optional
	TicketID string `json:"ticketID,omitempty"`
}

// Policy defines RBAC rules with optional namespace scoping.
type Policy struct {
	// Namespace scope for this policy. Empty means cluster-scoped.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// RBAC policy rules.
	// +kubebuilder:validation:MinItems=1
	Rules []rbacv1.PolicyRule `json:"rules"`
}

// ApprovalSpec defines approval configuration for the breakglass request.
type ApprovalSpec struct {
	// Required indicates whether manual approval is required.
	// +kubebuilder:default=true
	Required bool `json:"required"`
}

// ScheduleSpec defines the timing for breakglass activation.
type ScheduleSpec struct {
	// Start time (RFC3339 format) when schedule becomes active used for oneshots.
	// +optional
	Start metav1.Time `json:"start,omitempty"`

	// Duration after which access is revoked. If omitted
	// +optional
	Duration metav1.Duration `json:"duration"`

	// Optional cron schedule for recurring activations (min hour dom month ).
	// If omitted, the schedule is a one-time activation.
	// +optional
	Cron string `json:"cron,omitempty"`

	// Time zone location (IANA format, e.g., "America/New_York"). Defaults to UTC.
	// +optional
	Location string `json:"location,omitempty"`

	// Maximum activations. Schedule stops after reaching this count.
	// Only applicable if Cron is set.
	// +optional
	MaxActivations *int32 `json:"maxActivations,omitempty"`
}
