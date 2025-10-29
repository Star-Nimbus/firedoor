package metrics

type Op string

const (
	OpCreate     Op = "create"
	OpDelete     Op = "delete"
	OpReconcile  Op = "reconcile"
	OpValidation Op = "validation"
	OpRevoke     Op = "revoke"
	OpAlert      Op = "alert"
)

type Result string

const (
	ResultSuccess Result = "success"
	ResultError   Result = "error"
)

type Component string

const (
	ComponentController Component = "controller"
	ComponentWebhook    Component = "webhook"
	ComponentAPI        Component = "api"
)

type RoleType string

const (
	RoleTypeClusterRole RoleType = "cluster_role"
	RoleTypeRole        RoleType = "role"
	RoleTypeCustom      RoleType = "custom"
)
