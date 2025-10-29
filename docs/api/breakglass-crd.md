# Breakglass CRD Reference

Complete reference for the Breakglass Custom Resource Definition.

## API Version

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
```

## Schema

### Breakglass

| Field | Type | Description |
|-------|------|-------------|
| `apiVersion` | string | `access.cloudnimbus.io/v1alpha1` |
| `kind` | string | `Breakglass` |
| `metadata` | [ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta) | Standard Kubernetes metadata |
| `spec` | [BreakglassSpec](#breakglassspec) | Specification of the breakglass |
| `status` | [BreakglassStatus](#breakglassstatus) | Status of the breakglass |

### BreakglassSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `approval` | [ApprovalSpec](#approvalspec) | No | Approval configuration |
| `clusterRoles` | []string | No | List of cluster roles to grant |
| `policy` | [[]Policy](#policy) | No | Namespace-specific policies |
| `schedule` | [ScheduleSpec](#schedulespec) | Yes | Scheduling configuration |

### ApprovalSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `required` | boolean | Yes | Whether approval is required |
| `approvers` | []string | No | List of approver email addresses |

### ScheduleSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `cron` | string | No | Cron expression for recurring access |
| `duration` | [Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta) | Yes | Duration of access |
| `end` | [Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta) | No | End time for recurring access |
| `location` | string | No | Timezone for scheduling |
| `maxActivations` | int32 | No | Maximum number of activations |
| `start` | [Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta) | No | Start time for recurring access |

### Policy

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `rules` | [[]PolicyRule](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#policyrule-v1-rbac) | Yes | RBAC rules for the namespace |

### BreakglassStatus

| Field | Type | Description |
|-------|------|-------------|
| `activationCount` | int32 | Number of times access has been activated |
| `conditions` | [[]Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#condition-v1-meta) | Current conditions |
| `expiresAt` | [Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta) | When access expires |
| `grantedAt` | [Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta) | When access was granted |
| `nextActivationAt` | [Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta) | Next activation time for recurring access |

## Condition Types

| Type | Description |
|------|-------------|
| `Approved` | Access has been approved |
| `Expired` | Access has expired |
| `Failed` | An error occurred |
| `RecurringActive` | Recurring access is active |
| `RecurringPending` | Recurring access is pending |

## Condition Reasons

| Reason | Description |
|--------|-------------|
| `AccessExpired` | Access has expired |
| `AccessGranted` | Access has been granted |
| `AccessRevoked` | Access has been revoked |
| `ManualApproval` | Manually approved |
| `MaxActivationsReached` | Maximum activations reached |
| `RBACForbidden` | RBAC operation forbidden |
| `RBACTimeout` | RBAC operation timed out |
| `RecurringActivated` | Recurring access activated |
| `RecurringInvalidSchedule` | Invalid recurring schedule |
| `RecurringScheduled` | Recurring access scheduled |
| `RecurringWaiting` | Recurring access waiting |
| `RevokeFailed` | Revocation failed |
| `RoleBindingFailed` | Role binding creation failed |

## Examples

### Minimal Example

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: simple-access
spec:
  schedule:
    duration: "1h"
  clusterRoles:
    - admin
```

### Complete Example

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: complete-example
  namespace: default
  labels:
    environment: production
    team: platform
spec:
  approval:
    required: true
    approvers:
      - "admin@company.com"
      - "security@company.com"
  clusterRoles:
    - admin
    - view
  policy:
    - namespace: "production"
      rules:
        - apiGroups: ["apps"]
          resources: ["deployments"]
          verbs: ["get", "list", "update", "patch"]
        - apiGroups: [""]
          resources: ["pods"]
          verbs: ["get", "list", "watch"]
  schedule:
    cron: "0 9 * * 1-5"
    duration: "8h"
    start: "2024-01-01T00:00:00Z"
    end: "2024-12-31T23:59:59Z"
    location: "America/New_York"
    maxActivations: 10
status:
  activationCount: 3
  grantedAt: "2024-01-15T09:00:00Z"
  expiresAt: "2024-01-15T17:00:00Z"
  nextActivationAt: "2024-01-16T09:00:00Z"
  conditions:
    - type: "Approved"
      status: "True"
      reason: "ManualApproval"
      message: "Approved by admin@company.com"
      lastTransitionTime: "2024-01-15T08:55:00Z"
    - type: "RecurringActive"
      status: "True"
      reason: "RecurringActivated"
      message: "Recurring breakglass activated"
      lastTransitionTime: "2024-01-15T09:00:00Z"
```

## Validation Rules

### Required Fields

- `spec.schedule.duration` must be specified
- At least one of `spec.clusterRoles` or `spec.policy` must be specified

### Duration Validation

- `spec.schedule.duration` must be a positive duration
- Maximum duration is 24 hours for one-time access
- No maximum duration for recurring access

### Cron Validation

- `spec.schedule.cron` must be a valid cron expression
- Supports 5-field cron format: `minute hour day month weekday`
- Supports standard cron special characters: `*`, `,`, `-`, `/`

### Timezone Validation

- `spec.schedule.location` must be a valid IANA timezone
- Examples: `"UTC"`, `"America/New_York"`, `"Europe/London"`

### Approval Validation

- `spec.approval.required` must be `true` or `false`
- `spec.approval.approvers` must contain valid email addresses when specified

## Status Conditions

The status conditions provide detailed information about the current state of the breakglass:

```yaml
status:
  conditions:
    - type: "Approved"
      status: "True"  # or "False"
      reason: "ManualApproval"
      message: "Approved by admin@company.com"
      lastTransitionTime: "2024-01-15T08:55:00Z"
      observedGeneration: 1
```

### Condition Status Values

- `"True"`: The condition is satisfied
- `"False"`: The condition is not satisfied
- `"Unknown"`: The condition status is unknown

### Common Condition Transitions

1. **Pending → Approved**: When approval is granted
2. **Approved → RecurringActive**: When recurring access activates
3. **RecurringActive → Expired**: When access expires
4. **Any → Failed**: When an error occurs

## Error Handling

### Common Errors

| Error | Description | Resolution |
|-------|-------------|------------|
| `InvalidCronExpression` | Invalid cron expression | Fix the cron syntax |
| `InvalidTimezone` | Invalid timezone | Use valid IANA timezone |
| `InvalidDuration` | Invalid duration | Use positive duration |
| `RBACError` | RBAC operation failed | Check permissions |
| `ApprovalRequired` | Approval required but not granted | Grant approval |

### Error Recovery

Most errors are transient and will be retried automatically. For persistent errors:

1. Check the breakglass status and conditions
2. Review the operator logs
3. Verify RBAC permissions
4. Check for conflicting resources
