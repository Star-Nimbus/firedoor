# Core Concepts

Understanding the key concepts behind Firedoor will help you use it effectively.

## Breakglass

A **Breakglass** is a temporary access grant that provides elevated permissions for a limited time. Think of it as a "break glass in case of emergency" mechanism for Kubernetes access.

### Key Characteristics

- **Time-limited**: Access automatically expires after a specified duration
- **Auditable**: All access events are logged and tracked
- **Controlled**: Requires approval (configurable) before access is granted
- **Secure**: Integrates with Kubernetes RBAC for fine-grained permissions

## Access Types

### One-Time Access

Single-use access that expires after a specified duration:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: emergency-fix
spec:
  schedule:
    duration: "2h"
  clusterRoles:
    - admin
```

### Recurring Access

Regular access windows for maintenance or operations:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: weekly-maintenance
spec:
  schedule:
    cron: "0 2 * * 0"  # Every Sunday at 2 AM
    duration: "4h"
  clusterRoles:
    - cluster-admin
```

## Scheduling

Firedoor supports flexible scheduling options:

### Duration-Based

Simple time-limited access:

```yaml
schedule:
  duration: "1h"  # Access expires in 1 hour
```

### Cron-Based

Recurring access using cron expressions:

```yaml
schedule:
  cron: "0 9 * * 1-5"  # Weekdays at 9 AM
  duration: "8h"
  start: "2024-01-01T00:00:00Z"
  end: "2024-12-31T23:59:59Z"
```

### Timezone Support

Specify timezone for accurate scheduling:

```yaml
schedule:
  cron: "0 9 * * 1-5"
  location: "America/New_York"
  duration: "8h"
```

## Permissions

### Cluster Roles

Grant cluster-wide permissions:

```yaml
spec:
  clusterRoles:
    - admin
    - cluster-admin
    - view
```

### Namespace-Specific Permissions

Fine-grained control with custom policies:

```yaml
spec:
  policy:
    - namespace: "production"
      rules:
        - apiGroups: ["apps"]
          resources: ["deployments"]
          verbs: ["get", "list", "update"]
    - namespace: "staging"
      rules:
        - apiGroups: ["*"]
          resources: ["*"]
          verbs: ["*"]
```

## Approval Process

### Automatic Approval

For trusted scenarios:

```yaml
spec:
  approval:
    required: false
```

### Manual Approval

Require explicit approval before access is granted:

```yaml
spec:
  approval:
    required: true
    approvers:
      - "admin@company.com"
      - "security@company.com"
```

## Lifecycle States

A breakglass goes through several states:

1. **Pending**: Created but not yet approved (if approval required)
2. **Approved**: Approved and ready for activation
3. **Active**: Currently granting access
4. **Expired**: Access has expired
5. **Failed**: Error occurred during processing

## Monitoring and Observability

### Events

Firedoor emits Kubernetes events for all state changes:

```bash
kubectl get events --field-selector involvedObject.kind=Breakglass
```

### Metrics

Built-in Prometheus metrics:

- `firedoor_breakglass_total`: Total number of breakglasses
- `firedoor_breakglass_active`: Currently active breakglasses
- `firedoor_access_granted_total`: Total access grants
- `firedoor_access_revoked_total`: Total access revocations

### Logs

Structured logging with correlation IDs:

```bash
kubectl logs -l app.kubernetes.io/name=firedoor | grep "breakglass"
```

## Security Considerations

### Principle of Least Privilege

- Grant only the minimum permissions needed
- Use namespace-specific policies when possible
- Regularly review and audit access patterns

### Time Limits

- Set appropriate duration limits
- Use recurring schedules for regular maintenance
- Implement maximum activation limits for recurring access

### Audit Trail

- All access events are logged
- Integration with SIEM systems
- Compliance reporting capabilities

## Best Practices

### Naming Conventions

Use descriptive names that indicate purpose:

```yaml
metadata:
  name: "emergency-database-access-2024-01-15"
  # or
  name: "weekly-maintenance-window"
```

### Resource Organization

Group related breakglasses:

```yaml
metadata:
  name: "prod-emergency-access"
  namespace: "breakglass-system"
  labels:
    environment: "production"
    type: "emergency"
```

### Monitoring

- Set up alerts for failed breakglasses
- Monitor access patterns for anomalies
- Regular review of active breakglasses

## Integration Points

### CI/CD Pipelines

Integrate with deployment pipelines:

```yaml
# In your CI/CD pipeline
kubectl apply -f breakglass-maintenance.yaml
# ... perform maintenance ...
kubectl delete breakglass maintenance-window
```

### Monitoring Systems

- Prometheus metrics for alerting
- Grafana dashboards for visualization
- Integration with existing monitoring tools

### Identity Providers

- Integration with OIDC providers
- LDAP/Active Directory integration
- Multi-factor authentication support
