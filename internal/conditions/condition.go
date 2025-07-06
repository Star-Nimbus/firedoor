package conditions

// Condition represents the type of a breakglass condition
type Condition string

const (
	// Approved indicates the breakglass request has been approved
	Approved Condition = "Approved"
	// Denied indicates the breakglass request has been denied
	Denied Condition = "Denied"
	// Active indicates the breakglass access is currently active
	Active Condition = "Active"
	// Expired indicates the breakglass access has expired
	Expired Condition = "Expired"
	// Revoked indicates the breakglass access has been revoked
	Revoked Condition = "Revoked"
	// RecurringPending indicates the recurring breakglass is pending next activation
	RecurringPending Condition = "RecurringPending"
	// RecurringActive indicates the recurring breakglass access is currently active
	RecurringActive Condition = "RecurringActive"
)
