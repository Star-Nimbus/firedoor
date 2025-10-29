# Breakglass API v1alpha1

This directory contains the Kubernetes Custom Resource Definition (CRD) for the Breakglass API.

## File Organization

The API is organized into separate files for better maintainability:

- **`breakglass_types.go`** - Core types: `Breakglass` and `BreakglassList`
- **`breakglass_spec.go`** - Specification types: `BreakglassSpec`, `ApprovalSpec`, `TimeboxSpec`, `RecurrenceSpec`
- **`breakglass_status.go`** - Status types: `BreakglassStatus`, `BreakglassPhase`, `BreakglassCondition`, `BreakglassConditionReason`
- **`zz_generated.deepcopy.go`** - Auto-generated deep copy methods
- **`groupversion_info.go`** - API group and version information

## Design Principles

### 1. Leverage Upstream RBAC Types

Instead of inventing custom types, we reuse the canonical Kubernetes RBAC types:

- `rbacv1.Subject` for users, groups, and service accounts
- `rbacv1.PolicyRule` for access rules
- This reduces code duplication and ensures compatibility

### 2. Flattened Logic-Centric Structure

Complex boolean flags are organized into dedicated sub-structs:

- `ApprovalSpec` for approval requirements
- `TimeboxSpec` for time constraints  
- `RecurrenceSpec` for recurring access patterns

### 3. Standard Kubernetes Condition Pattern

Uses the standard `metav1.Condition` pattern for state tracking:

- **Phase**: Simple enum for user-facing high-level state
- **Conditions**: Rich conditions for detailed status reporting and tooling interop
- **Condition Reasons**: Specific reasons for condition states using `BreakglassConditionReason`
- Consistent with Kubernetes conventions using `meta.SetStatusCondition` and `meta.FindStatusCondition`

### 4. Optional Fields Done Right

- Use pointers (`*bool`, `*metav1.Duration`) only when distinguishing "not set" from "false/zero"
- Value fields for common cases to avoid nil checks
- Default values where appropriate

## Usage Examples

### Basic Breakglass Request

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: emergency-access
spec:
  subjects:
  - kind: User
    name: admin@example.com
  clusterRoles:
  - cluster-admin
  approval:
    required: true
  justification: "Production outage - need emergency access to restart services"
  schedule:
    start: "2024-01-01T00:00:00Z"
    end: "2024-01-01T02:00:00Z"
```

### Custom Policy Breakglass

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: custom-policy-access
spec:
  subjects:
  - kind: User
    name: developer@example.com
  inlinePolicy:
  - verbs: ["get", "list", "watch"]
    apiGroups: [""]
    resources: ["pods", "services"]
    namespaces: ["default", "production"]
  justification: "Debug production issue - need to inspect pod logs"
  schedule:
    start: "2024-01-01T00:00:00Z"
    end: "2024-01-01T00:30:00Z"
```

### Recurring Access

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: weekend-maintenance
spec:
  subjects:
  - kind: Group
    name: maintenance-team
  clusterRoles:
  - cluster-admin
  approval:
    required: false
  justification: "Weekend maintenance window - automated access"
  schedule:
    start: "2024-01-01T00:00:00Z"
    end: "2024-01-01T08:00:00Z"
```

## Migration Notes

When migrating from older versions:

- `AccessPolicy` → `InlinePolicy`
- `ApprovalRequired` → `Approval.Required`
- `Duration` → `Timebox.Duration`
- `Recurring` → `Recurrence.Enabled`
- `RecurrenceSchedule` → `Recurrence.Schedule`
- `SubjectRef` → `rbacv1.Subject`
- `AccessRule` → `rbacv1.PolicyRule`
