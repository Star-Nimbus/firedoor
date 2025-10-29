package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BreakglassCondition represents the type of a breakglass condition
type BreakglassCondition string

const (
	// NoCondition represents an unset or empty condition
	NoCondition BreakglassCondition = ""
	// ConditionPending indicates the breakglass request is pending
	ConditionPending BreakglassCondition = "Pending"
	// ConditionApproved indicates the breakglass request has been approved
	ConditionApproved BreakglassCondition = "Approved"
	// ConditionDenied indicates the breakglass request has been denied
	ConditionDenied BreakglassCondition = "Denied"
	// ConditionActive indicates the breakglass access is currently active
	ConditionActive BreakglassCondition = "Active"
	// ConditionExpired indicates the breakglass access has expired
	ConditionExpired BreakglassCondition = "Expired"
	// ConditionRevoked indicates the breakglass access has been revoked
	ConditionRevoked BreakglassCondition = "Revoked"
	// ConditionFailed indicates the breakglass request has failed
	ConditionFailed BreakglassCondition = "Failed"
	// ConditionRecurringPending indicates the recurring breakglass is waiting for next activation
	ConditionRecurringPending BreakglassCondition = "RecurringPending"
	// ConditionRecurringActive indicates the recurring breakglass is currently active
	ConditionRecurringActive BreakglassCondition = "RecurringActive"
)

// BreakglassConditionReason represents the reason for a breakglass condition
type BreakglassConditionReason string

const (
	// ReasonNewResource indicates the resource was just created
	ReasonNewResource BreakglassConditionReason = "NewResource"
	// ReasonWaitingForApproval indicates the request is waiting for approval
	ReasonWaitingForApproval BreakglassConditionReason = "WaitingForApproval"
	// ReasonAccessGranted indicates access was successfully granted
	ReasonAccessGranted BreakglassConditionReason = "AccessGranted"
	// ReasonAccessDenied indicates access was denied
	ReasonAccessDenied BreakglassConditionReason = "AccessDenied"
	// ReasonAccessActive indicates access is currently active
	ReasonAccessActive BreakglassConditionReason = "AccessActive"
	// ReasonAccessExpired indicates access has expired
	ReasonAccessExpired BreakglassConditionReason = "AccessExpired"
	// ReasonAccessRevoked indicates access was revoked
	ReasonAccessRevoked BreakglassConditionReason = "AccessRevoked"
	// ReasonRBACCreationFailed indicates RBAC creation failed
	ReasonRBACCreationFailed BreakglassConditionReason = "RBACCreationFailed"
	// ReasonRBACFOrbidden indicates RBAC operation was forbidden
	ReasonRBACForbidden BreakglassConditionReason = "RBACForbidden"
	// ReasonRBACTimeout indicates RBAC operation timed out
	ReasonRBACTimeout BreakglassConditionReason = "RBACTimeout"
	// ReasonInvalidRequest indicates the request was invalid
	ReasonInvalidRequest BreakglassConditionReason = "InvalidRequest"
	// ReasonRequestDeniedDueToMissingUserOrGroup indicates request was denied due to missing user or group
	ReasonRequestDeniedDueToMissingUserOrGroup BreakglassConditionReason = "RequestDeniedDueToMissingUserOrGroup"
	// ReasonBreakglassAccessExpiredAndRevoked indicates breakglass access expired and was revoked
	ReasonBreakglassAccessExpiredAndRevoked BreakglassConditionReason = "BreakglassAccessExpiredAndRevoked"
	// ReasonAccessIsNoLongerActive indicates access is no longer active
	ReasonAccessIsNoLongerActive BreakglassConditionReason = "AccessIsNoLongerActive"
	// ReasonRoleBindingFailed indicates the role binding creation failed
	ReasonRoleBindingFailed BreakglassConditionReason = "RoleBindingFailed"
	// ReasonRevokeFailed indicates the revocation operation failed
	ReasonRevokeFailed BreakglassConditionReason = "RevokeFailed"
	// ReasonRecurringScheduled indicates the recurring breakglass has been scheduled
	ReasonRecurringScheduled BreakglassConditionReason = "RecurringScheduled"
	// ReasonRecurringActivated indicates the recurring breakglass has been activated
	ReasonRecurringActivated BreakglassConditionReason = "RecurringActivated"
	// ReasonRecurringWaiting indicates the recurring breakglass is waiting for next activation
	ReasonRecurringWaiting BreakglassConditionReason = "RecurringWaiting"
	// ReasonRecurringInvalidSchedule indicates the recurring schedule is invalid
	ReasonRecurringInvalidSchedule BreakglassConditionReason = "RecurringInvalidSchedule"
	// ReasonMaxActivationsReached indicates the maximum number of activations has been reached
	ReasonMaxActivationsReached BreakglassConditionReason = "MaxActivationsReached"
)

// BreakglassStatus defines the observed state of Breakglass (set by the operator).
type BreakglassStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	GrantedAt  *metav1.Time       `json:"grantedAt,omitempty"`
	ExpiresAt  *metav1.Time       `json:"expiresAt,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ApprovedBy is the username or identity that approved the breakglass request.
	// +optional
	ApprovedBy string `json:"approvedBy,omitempty"`

	// CreatedResources tracks the names of RBAC resources created by this breakglass.
	// +optional
	CreatedResources []string `json:"createdResources,omitempty"`

	// Optional tracking for recurring requests.
	NextActivationAt *metav1.Time `json:"nextActivationAt,omitempty"`
	ActivationCount  int32        `json:"activationCount,omitempty"`
}

// String returns the string representation of the condition
func (c BreakglassCondition) String() string {
	return string(c)
}

// String returns the string representation of the reason
func (r BreakglassConditionReason) String() string {
	return string(r)
}
