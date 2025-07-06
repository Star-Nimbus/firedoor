package conditions

// Operation represents the type of operation being performed
type Operation string

const (
	// Create indicates a create operation
	Create Operation = "Create"
	// Update indicates an update operation
	Update Operation = "Update"
	// Delete indicates a delete operation
	Delete Operation = "Delete"
	// Grant indicates a grant operation
	Grant Operation = "Grant"
	// Revoke indicates a revoke operation
	Revoke Operation = "Revoke"
)
