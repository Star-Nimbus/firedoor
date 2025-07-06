# Breakglass Access Flow Example

This directory demonstrates a complete breakglass access request flow using Firedoor, from request creation to revocation.

## Flow Overview

1. **01-create-request.yaml** - Create a breakglass access request
2. **02-verify-status.yaml** - Template for verifying request status (not applied directly)
3. **03-revoke-access.yaml** - Revoke the breakglass access
4. **04-namespace-role-example.yaml** - Example with namespace-specific roles
5. **05-recurring-breakglass-example.yaml** - Example of recurring breakglass access
6. **06-alertmanager-config.yaml** - Optional Alertmanager integration configuration

## CRD Structure

The Breakglass CRD has the following structure:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: <name>
  namespace: <namespace>
spec:
  # Subject configuration - either user or group must be provided
  user: <user-email>  # or group: <group-name>
  
  # Role configuration
  role: <role-name>  # Name of Role or ClusterRole to bind
  
  # Namespace for RoleBinding (empty for ClusterRoleBinding)
  namespace: <namespace>  # Empty for cluster-wide access
  
  # Duration in minutes for how long the access is granted
  durationMinutes: <minutes>
  
  # Human-readable reason for the access
  reason: <reason>
  
  # Must be true for the access to be granted
  approved: <boolean>
```

## Prerequisites

- Kubernetes cluster with Firedoor operator installed
- `kubectl` configured to access the cluster
- `make` installed

## Usage

### Quick Start

Run the complete flow:

```bash
make run-flow
```

### Step by Step

1. **Create the breakglass request:**

   ```bash
   make create-request
   ```

2. **Check the request status:**

   ```bash
   make check-status
   ```

3. **Revoke the access:**

   ```bash
   make revoke-access
   ```

4. **Clean up:**

   ```bash
   make cleanup
   ```

## Available Make Targets

- `create-request` - Apply the breakglass request
- `check-status` - Check the current status of the request
- `revoke-access` - Revoke the breakglass access
- `cleanup` - Remove the breakglass resource
- `run-flow` - Execute the complete flow with delays
- `show-logs` - Show Firedoor operator logs
- `describe` - Describe the breakglass resource

## Example Scenarios

### Emergency Database Maintenance

This example shows an emergency database maintenance scenario where:

1. An emergency admin requests cluster-admin access
2. The request is auto-approved (approved: true)
3. Access is granted for 2 hours (durationMinutes: 120)
4. Access is revoked using annotations

### Manual Approval Flow

To test manual approval, modify `01-create-request.yaml`:

```yaml
spec:
  # ... other fields ...
  approved: false  # Requires manual approval
```

## Status Phases

The Breakglass resource can have the following phases:

- **Pending** - Request is waiting for approval
- **Active** - Access is currently granted
- **Expired** - Access has expired
- **Denied** - Request was denied
- **Revoked** - Access was manually revoked

## Revocation

Access can be revoked by adding annotations to the Breakglass resource:

```yaml
metadata:
  annotations:
    firedoor.cloudnimbus.io/revoke: "true"
    firedoor.cloudnimbus.io/revoke-reason: "Reason for revocation"
    firedoor.cloudnimbus.io/revoked-by: "user@example.com"
    firedoor.cloudnimbus.io/revoked-at: "2024-01-15T11:45:00Z"
```

## Monitoring and Auditing

The Firedoor operator provides:

- **Audit logs** - All breakglass operations are logged
- **Metrics** - Prometheus metrics for monitoring
- **Events** - Kubernetes events for each operation
- **Status conditions** - Real-time status updates

## Security Considerations

- All breakglass requests are audited
- Access is automatically revoked after the specified duration
- Manual revocation is available via annotations
- All operations are logged for compliance

## Troubleshooting

### Check if Firedoor is running

```bash
kubectl get pods -n firedoor-system
```

### View operator logs

```bash
make show-logs
```

### Check breakglass resource

```bash
make describe
```

### Common Issues

1. **CRD not installed** - Ensure Firedoor CRDs are installed
2. **Operator not running** - Check if the Firedoor operator is deployed
3. **Permission denied** - Verify RBAC permissions are configured
4. **Request not approved** - Check the `approved` field in the spec

## Alertmanager Integration (Optional)

The Firedoor operator includes optional integration with Alertmanager for sending alerts when breakglass access becomes active or expires. This feature is **disabled by default** and must be explicitly enabled.

### Enabling Alertmanager Integration

To enable Alertmanager integration, set the `alertmanager.enabled` flag to `true` in your configuration:

```yaml
alertmanager:
  enabled: true
  url: "http://alertmanager.telemetry-system.svc.cluster.local:9093"
  timeout: 30s
  
  # Optional authentication
  basicAuth:
    username: "alertmanager-user"
    password: "alertmanager-password"
  
  # Optional TLS configuration
  tls:
    insecureSkipVerify: false
    caFile: "/path/to/ca.crt"
    certFile: "/path/to/cert.crt"
    keyFile: "/path/to/key.key"
  
  # Alert configuration
  alert:
    alertName: "BreakglassActive"
    severity: "warning"
    summary: "Breakglass access is active"
    description: "A breakglass access request is currently active"
    labels:
      team: "platform"
      component: "firedoor"
    annotations:
      runbook_url: "https://wiki.company.com/runbooks/breakglass-access"
```

### Helm Configuration

When using Helm, you can enable Alertmanager integration by setting:

```bash
helm install firedoor oci://ghcr.io/cloud-nimbus/firedoor/charts/firedoor \
  --set alertmanager.enabled=true \
  --set alertmanager.url="http://alertmanager.telemetry-system.svc.cluster.local:9093"
```

### Behavior When Disabled

When Alertmanager integration is disabled (the default):

- No HTTP client is created
- Alert sending operations return immediately with no error
- No network connections are attempted
- The operator functions normally without any alerting overhead

### Behavior When Enabled

When Alertmanager integration is enabled:

- Alerts are sent when breakglass access becomes active
- Alerts are sent when breakglass access expires
- Failed alert sends are logged but don't affect breakglass operations
- Telemetry is recorded for successful and failed alert operations

### Alert Format

Alerts sent to Alertmanager include:

**Labels:**

- `alertname`: The configured alert name (default: "BreakglassActive")
- `severity`: The configured severity level (default: "warning")
- `breakglass_name`: The name of the breakglass resource
- `breakglass_namespace`: The namespace of the breakglass resource
- `status`: "active" or "expired"

**Annotations:**

- `summary`: Alert summary
- `description`: Alert description
- `justification`: The breakglass justification
- `approved_by`: Who approved the breakglass
- `subjects`: List of subjects granted access
- `ticket_id`: Associated ticket ID (if provided)
- `expires_at`: When the access expires (for active alerts)
- `granted_at`: When access was granted (for expired alerts)

See `06-alertmanager-config.yaml` for a complete example configuration.
