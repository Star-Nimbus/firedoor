# Quick Start

Get up and running with Firedoor in 5 minutes!

## Prerequisites

- Firedoor installed in your cluster (see [Installation](installation.md))
- kubectl configured to access your cluster
- A user with cluster-admin permissions (for this example)

## Your First Breakglass

Let's create a simple one-time breakglass access:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: my-first-breakglass
  namespace: default
spec:
  schedule:
    duration: "30m"  # Access expires in 30 minutes
  clusterRoles:
    - admin
```

Save this as `my-breakglass.yaml` and apply it:

```bash
kubectl apply -f my-breakglass.yaml
```

## Check the Status

View your breakglass:

```bash
kubectl get breakglasses
kubectl describe breakglass my-first-breakglass
```

You should see the breakglass in `Pending` status, waiting for approval.

## Approve the Access

Approve the breakglass to grant access:

```bash
kubectl patch breakglass my-first-breakglass --type='merge' -p='{"spec":{"approval":{"required":false}}}'
```

Or if approval is required, use:

```bash
kubectl patch breakglass my-first-breakglass --type='merge' -p='{"status":{"conditions":[{"type":"Approved","status":"True","reason":"ManualApproval","message":"Approved by admin"}]}}'
```

## Verify Access

Check that the access was granted:

```bash
kubectl get breakglasses
kubectl get rolebindings
kubectl get clusterrolebindings
```

You should see:

- Breakglass status: `Active`
- New RoleBinding/ClusterRoleBinding created
- Access granted for the specified duration

## Clean Up

The access will automatically expire after 30 minutes, but you can also revoke it manually:

```bash
kubectl delete breakglass my-first-breakglass
```

## Next: Recurring Access

Let's create a recurring breakglass for regular maintenance:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: maintenance-window
  namespace: default
spec:
  schedule:
    cron: "0 2 * * 0"  # Every Sunday at 2 AM
    duration: "4h"      # 4-hour maintenance window
    start: "2024-01-01T00:00:00Z"
  clusterRoles:
    - cluster-admin
  approval:
    required: false
```

Apply and monitor:

```bash
kubectl apply -f maintenance-breakglass.yaml
kubectl get breakglasses -w
```

## What's Next?

- [Configuration Guide](configuration.md) - Learn about all configuration options
- [User Guide](../user-guide/concepts.md) - Understand core concepts
- [Examples](../examples/basic-usage.md) - See more complex examples
- [API Reference](../api/breakglass-crd.md) - Complete API documentation

## Common Commands

```bash
# List all breakglasses
kubectl get breakglasses

# Get detailed information
kubectl describe breakglass <name>

# Watch for changes
kubectl get breakglasses -w

# Check events
kubectl get events --sort-by=.metadata.creationTimestamp

# View logs
kubectl logs -l app.kubernetes.io/name=firedoor
```

## Troubleshooting

If something doesn't work as expected:

1. **Check the operator logs:**

   ```bash
   kubectl logs -l app.kubernetes.io/name=firedoor
   ```

2. **Verify RBAC permissions:**

   ```bash
   kubectl auth can-i create breakglasses
   kubectl auth can-i create rolebindings
   ```

3. **Check the breakglass status:**

   ```bash
   kubectl describe breakglass <name>
   ```

4. **Look for events:**

   ```bash
   kubectl get events --field-selector involvedObject.name=<breakglass-name>
   ```
