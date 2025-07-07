/*
Copyright 2025.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BreakglassSpec defines the desired state of a Breakglass access request.
type BreakglassSpec struct {
	// Subjects defines the users or groups to grant temporary access to.
	// Each subject is specified by kind (User, Group, or ServiceAccount) and name.
	// Only Kubernetes RBAC subjects are supported (e.g., User or Group names as known to the cluster).
	// +kubebuilder:validation:MinItems=1
	Subjects []SubjectRef `json:"subjects,omitempty"`

	// AccessPolicy specifies inline RBAC rules (permissions) to grant.
	// If provided, the operator will create a temporary Role/ClusterRole with these rules.
	// +optional
	AccessPolicy *AccessPolicy `json:"accessPolicy,omitempty"`

	// ClusterRoles lists existing ClusterRole names to bind to the subjects for the duration.
	// Use this to grant pre-defined roles (e.g., "cluster-admin") without specifying custom rules.
	// +optional
	ClusterRoles []string `json:"clusterRoles,omitempty"`

	// ApprovalRequired indicates if manual approval by a privileged user is needed before access is activated.
	// Recommended to default to true for safety.
	// +kubebuilder:default=true
	ApprovalRequired bool `json:"approvalRequired,omitempty"`

	// Duration is the requested length of time that access should be active (e.g., "1h", "30m").
	// After this duration, the access will expire automatically.
	// Uses Kubernetes duration format (Go metav1.Duration).
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// Justification is a **required** explanation of why this access is needed.
	// This should be a clear, specific reason and may be logged for audit purposes.
	// (e.g., "Production outage troubleshooting - need access to restart pods").
	// +kubebuilder:validation:MinLength=1
	Justification string `json:"justification"`

	// TicketID (optional) links to an external incident or change ticket for this request.
	// This provides traceability to external audit systems.
	// +optional
	TicketID string `json:"ticketID,omitempty"`

	// Recurring indicates if the access request is recurring (e.g., a standing approval for repeated use).
	// If true, the operator may allow this access to be reactivated on a schedule or manually without a new request.
	// +optional
	Recurring bool `json:"recurring,omitempty"`

	// RecurrenceSchedule defines an optional schedule (cron expression or interval) for recurring access activations.
	// For example, a cron string to allow access every weekday at 9AM. This field is used only if Recurring is true.
	// +optional
	RecurrenceSchedule string `json:"recurrenceSchedule,omitempty"`
}

// SubjectRef represents a subject (user, group, or service account) for RBAC.
type SubjectRef struct {
	// Kind of subject (e.g., "User", "Group", or "ServiceAccount").
	Kind string `json:"kind"`
	// Name of the user, group, or service account.
	Name string `json:"name"`
	// Namespace for ServiceAccount subjects (optional; not used for User or Group kinds).
	Namespace string `json:"namespace,omitempty"`
}

// AccessPolicy defines a set of RBAC rules (permissions) to grant.
type AccessPolicy struct {
	// Rules is the list of access rules (verbs, resources, etc.) that define the permissions.
	// Each rule is analogous to a single RBAC policy rule.
	// +optional
	Rules []AccessRule `json:"rules,omitempty"`
}

// Action (aka Verb) defines valid Kubernetes API actions for a rule.
// +kubebuilder:validation:Enum=get;list;create;update;patch;delete;watch
type Action string

//go:generate stringer -type=Action

const (
	ActionGet    Action = "get"
	ActionList   Action = "list"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionPatch  Action = "patch"
	ActionDelete Action = "delete"
	ActionWatch  Action = "watch"
)

// String lets fmt, log, klog, etc. print the value naturally.
func (a Action) String() string { return string(a) }

// AccessRule describes a single permission rule to be granted.
type AccessRule struct {
	// Actions (verbs) allowed, e.g., ["get", "list", "update"].
	Actions []Action `json:"actions,omitempty"`
	// APIGroups of the resources (e.g., ["", "apps"] for core or named API groups).
	// Use "" for core API group resources.
	// +optional
	APIGroups []string `json:"apiGroups,omitempty"`
	// Resources to which the actions apply (e.g., ["pods", "deployments"]).
	// Subresources can be specified as "resource/subresource" (e.g., "pods/log").
	// +optional
	Resources []string `json:"resources,omitempty"`
	// ResourceNames restricts the rule to specific resource instances by name (e.g., ["my-configmap"]).
	// If empty or unspecified, the rule applies to all objects of the given resource types.
	// +optional
	ResourceNames []string `json:"resourceNames,omitempty"`
	// Namespaces to which this rule applies. If specified, access is limited to these namespaces.
	// If empty, the rule applies cluster-wide or to cluster-scoped resources.
	// (For cluster-scoped resources like "nodes", leave this empty or use cluster-wide bindings.)
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`
}

// BreakglassPhase represents the lifecycle state of a Breakglass request.
type BreakglassPhase string

const (
	PhasePending BreakglassPhase = "Pending" // Request created, waiting for approval.
	PhaseActive  BreakglassPhase = "Active"  // Access is approved/granted and currently active.
	PhaseExpired BreakglassPhase = "Expired" // Access duration ended naturally.
	PhaseDenied  BreakglassPhase = "Denied"  // Request was denied by an approver.
	PhaseRevoked BreakglassPhase = "Revoked" // Access was manually revoked before expiry.
	// Recurring phases
	PhaseRecurringPending BreakglassPhase = "RecurringPending" // Recurring request is pending next activation.
	PhaseRecurringActive  BreakglassPhase = "RecurringActive"  // Recurring access is currently active.
)

// BreakglassStatus defines the observed state of Breakglass (set by the operator).
type BreakglassStatus struct {
	// Phase is the current state of the request (Pending, Active, Expired, Denied, or Revoked).
	Phase BreakglassPhase `json:"phase,omitempty"`
	// ApprovedAt is the timestamp when the request was approved. Nil if not yet approved or if auto-approved.
	// +optional
	ApprovedAt *metav1.Time `json:"approvedAt,omitempty"`
	// ApprovedBy records the username or identity of the approver who approved the request.
	// For auto-approved requests, this could be set to a system identity.
	// +optional
	ApprovedBy string `json:"approvedBy,omitempty"`
	// GrantedAt is the timestamp when access was actually granted (activated).
	// In many cases this will equal ApprovedAt, but if there is a delay or scheduled start, it may differ.
	// +optional
	GrantedAt *metav1.Time `json:"grantedAt,omitempty"`
	// ExpiresAt is the timestamp when the access is scheduled to expire (GrantedAt + Duration).
	// The operator should revoke access at or after this time if the request is active.
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`
	// NextActivationAt is the timestamp when the next recurring activation should occur.
	// This is calculated based on the RecurrenceSchedule and is only set for recurring breakglass requests.
	// +optional
	NextActivationAt *metav1.Time `json:"nextActivationAt,omitempty"`
	// ActivationCount tracks the number of times this recurring breakglass has been activated.
	// This helps with monitoring and debugging recurring access patterns.
	// +optional
	ActivationCount int32 `json:"activationCount,omitempty"`
	// LastActivationAt is the timestamp of the most recent activation for recurring breakglass requests.
	// +optional
	LastActivationAt *metav1.Time `json:"lastActivationAt,omitempty"`
	// Conditions represent the current conditions of the request (for Kubernetes standard Condition reporting).
	// Example conditions might include "Approved" or "Expired" with True/False status.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Breakglass is the Schema for the breakglass API, representing a single emergency access request.
type Breakglass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BreakglassSpec   `json:"spec,omitempty"`
	Status BreakglassStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BreakglassList contains a list of Breakglass requests.
type BreakglassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Breakglass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Breakglass{}, &BreakglassList{})
}
