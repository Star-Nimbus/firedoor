package conditions

// Reason represents the reason for a breakglass condition
type Reason string

const (
	// AccessGranted indicates access was successfully granted
	AccessGranted Reason = "AccessGranted"
	// AccessDenied indicates access was denied
	AccessDenied Reason = "AccessDenied"
	// AccessActive indicates access is currently active
	AccessActive Reason = "AccessActive"
	// AccessExpired indicates access has expired
	AccessExpired Reason = "AccessExpired"
	// AccessRevoked indicates access was revoked
	AccessRevoked Reason = "AccessRevoked"
	// InvalidRequest indicates the request was invalid
	InvalidRequest Reason = "InvalidRequest"
	// RoleBindingFailed indicates the role binding creation failed
	RoleBindingFailed Reason = "RoleBindingFailed"
	// RevokeFailed indicates the revocation operation failed
	RevokeFailed Reason = "RevokeFailed"
)
