package conditions

// Message represents the message for a breakglass condition
type Message string

const (
	// RequestDeniedDueToMissingUserOrGroup indicates the request was denied due to missing user or group
	RequestDeniedDueToMissingUserOrGroup Message = "Request denied due to missing user or group"
	// AccessDeniedDueToRoleBindingFailure indicates access was denied due to role binding failure
	AccessDeniedDueToRoleBindingFailure Message = "Access denied due to role binding failure"
	// BreakglassAccessExpiredAndRevoked indicates the breakglass access has expired and been revoked
	BreakglassAccessExpiredAndRevoked Message = "Breakglass access expired and revoked"
	// AccessIsNoLongerActive indicates the access is no longer active
	AccessIsNoLongerActive Message = "Access is no longer active"
)
