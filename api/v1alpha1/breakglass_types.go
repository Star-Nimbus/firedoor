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

// BreakglassSpec defines the desired state of Breakglass
type BreakglassSpec struct {
	// One of: user or group must be provided
	User  string `json:"user,omitempty"`
	Group string `json:"group,omitempty"`

	// Name of Role or ClusterRole to bind
	Role string `json:"role"`

	// Namespace for RoleBinding (empty for ClusterRoleBinding)
	Namespace string `json:"namespace"`

	// Duration in minutes for how long the access is granted
	DurationMinutes int `json:"durationMinutes"`

	// Human-readable reason for the access
	Reason string `json:"reason,omitempty"`

	// Must be true for the access to be granted
	Approved bool `json:"approved"`
}
type BreakglassPhase string

const (
	PhasePending BreakglassPhase = "Pending"
	PhaseActive  BreakglassPhase = "Active"
	PhaseExpired BreakglassPhase = "Expired"
	PhaseDenied  BreakglassPhase = "Denied"
	PhaseRevoked BreakglassPhase = "Revoked"
)

// BreakglassStatus defines the observed state of Breakglass
type BreakglassStatus struct {
	// +kubebuilder:validation:Enum=Pending;Active;Expired;Denied;Revoked
	// Phase of the request (e.g. Pending, Active, Expired, Denied)
	Phase *BreakglassPhase `json:"phase,omitempty"`
	// Timestamp when the access will expire
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// Timestamp when the access was granted
	GrantedAt *metav1.Time `json:"grantedAt,omitempty"`

	// Who approved the request (optional, set by dashboard/backend)
	ApprovedBy string `json:"approvedBy,omitempty"`

	// Conditions to track the status of the breakglass request more verbosely
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Breakglass is the Schema for the breakglass API
type Breakglass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BreakglassSpec   `json:"spec,omitempty"`
	Status BreakglassStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BreakglassList contains a list of Breakglass
type BreakglassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Breakglass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Breakglass{}, &BreakglassList{})
}
