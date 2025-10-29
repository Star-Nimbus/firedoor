# Basic Usage Examples

This page provides practical examples of how to use Firedoor in common scenarios.

## Emergency Access

### Quick Emergency Fix

When you need immediate access to fix a critical issue:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: emergency-fix-2024-01-15
  namespace: default
spec:
  schedule:
    duration: "1h"
  clusterRoles:
    - admin
  approval:
    required: false  # For true emergencies
```

### Emergency with Approval

For more controlled emergency access:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: emergency-database-access
  namespace: default
spec:
  schedule:
    duration: "30m"
  policy:
    - namespace: "production"
      rules:
        - apiGroups: ["apps"]
          resources: ["deployments", "pods"]
          verbs: ["get", "list", "update", "patch"]
  approval:
    required: true
    approvers:
      - "admin@company.com"
      - "security@company.com"
```

## Maintenance Windows

### Weekly Maintenance

Regular maintenance window every Sunday:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: weekly-maintenance
  namespace: default
spec:
  schedule:
    cron: "0 2 * * 0"  # Every Sunday at 2 AM
    duration: "4h"
    start: "2024-01-01T00:00:00Z"
  clusterRoles:
    - cluster-admin
  approval:
    required: false
```

### Daily Operations

Daily operational access for specific teams:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: daily-operations
  namespace: default
spec:
  schedule:
    cron: "0 9 * * 1-5"  # Weekdays at 9 AM
    duration: "8h"
    location: "America/New_York"
  policy:
    - namespace: "monitoring"
      rules:
        - apiGroups: ["*"]
          resources: ["*"]
          verbs: ["get", "list", "watch"]
    - namespace: "logging"
      rules:
        - apiGroups: ["*"]
          resources: ["*"]
          verbs: ["get", "list", "watch"]
```

## Development Access

### Temporary Development Access

Short-term access for development work:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: dev-access-john
  namespace: default
spec:
  schedule:
    duration: "4h"
  policy:
    - namespace: "development"
      rules:
        - apiGroups: ["*"]
          resources: ["*"]
          verbs: ["*"]
    - namespace: "staging"
      rules:
        - apiGroups: ["*"]
          resources: ["*"]
          verbs: ["get", "list", "watch", "create", "update"]
  approval:
    required: true
    approvers:
      - "team-lead@company.com"
```

### Feature Branch Testing

Access for testing new features:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: feature-testing-v2.1
  namespace: default
spec:
  schedule:
    duration: "2h"
  policy:
    - namespace: "feature-testing"
      rules:
        - apiGroups: ["*"]
          resources: ["*"]
          verbs: ["*"]
  approval:
    required: false
```

## Production Support

### On-Call Access

Access for on-call engineers:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: oncall-access
  namespace: default
spec:
  schedule:
    cron: "0 18 * * 1-5"  # Weekdays at 6 PM
    duration: "12h"
  clusterRoles:
    - admin
  approval:
    required: false
```

### Database Maintenance

Specific access for database operations:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: database-maintenance
  namespace: default
spec:
  schedule:
    cron: "0 3 * * 0"  # Every Sunday at 3 AM
    duration: "2h"
  policy:
    - namespace: "production"
      rules:
        - apiGroups: ["apps"]
          resources: ["deployments", "statefulsets"]
          verbs: ["get", "list", "update", "patch"]
        - apiGroups: [""]
          resources: ["pods", "services", "configmaps"]
          verbs: ["get", "list", "update", "patch"]
  approval:
    required: true
    approvers:
      - "dba@company.com"
```

## Security Scenarios

### Security Incident Response

Access for security team during incidents:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: security-incident-response
  namespace: default
spec:
  schedule:
    duration: "6h"
  clusterRoles:
    - cluster-admin
  approval:
    required: true
    approvers:
      - "security@company.com"
      - "ciso@company.com"
```

### Compliance Audit

Access for compliance audits:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: compliance-audit-2024
  namespace: default
spec:
  schedule:
    duration: "8h"
  policy:
    - namespace: "production"
      rules:
        - apiGroups: ["*"]
          resources: ["*"]
          verbs: ["get", "list", "watch"]
  approval:
    required: true
    approvers:
      - "compliance@company.com"
      - "audit@company.com"
```

## Advanced Examples

### Multi-Environment Access

Access across multiple environments:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: cross-env-deployment
  namespace: default
spec:
  schedule:
    duration: "2h"
  policy:
    - namespace: "staging"
      rules:
        - apiGroups: ["*"]
          resources: ["*"]
          verbs: ["*"]
    - namespace: "production"
      rules:
        - apiGroups: ["apps"]
          resources: ["deployments"]
          verbs: ["get", "list", "update", "patch"]
  approval:
    required: true
    approvers:
      - "release-manager@company.com"
```

### Limited Resource Access

Very specific resource access:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: specific-resource-access
  namespace: default
spec:
  schedule:
    duration: "1h"
  policy:
    - namespace: "production"
      rules:
        - apiGroups: ["apps"]
          resources: ["deployments"]
          resourceNames: ["web-app", "api-server"]
          verbs: ["get", "update", "patch"]
  approval:
    required: true
    approvers:
      - "team-lead@company.com"
```

## Monitoring and Management

### Check Status

```bash
# List all breakglasses
kubectl get breakglasses

# Get detailed information
kubectl describe breakglass <name>

# Watch for changes
kubectl get breakglasses -w
```

### Approve Access

```bash
# Approve a breakglass
kubectl patch breakglass <name> --type='merge' -p='{"status":{"conditions":[{"type":"Approved","status":"True","reason":"ManualApproval","message":"Approved by admin"}]}}'
```

### Revoke Access

```bash
# Delete a breakglass (immediate revocation)
kubectl delete breakglass <name>
```

### Check Events

```bash
# View all breakglass events
kubectl get events --field-selector involvedObject.kind=Breakglass

# View events for specific breakglass
kubectl get events --field-selector involvedObject.name=<breakglass-name>
```

## Best Practices

1. **Use descriptive names** that indicate purpose and date
2. **Set appropriate durations** - not too short, not too long
3. **Require approval** for production access
4. **Use namespace-specific policies** when possible
5. **Monitor and audit** all access
6. **Clean up** expired breakglasses regularly
7. **Document** the purpose of each breakglass
8. **Test** breakglass configurations in non-production first
