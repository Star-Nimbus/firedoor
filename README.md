# Firedoor

Firedoor is a Kubernetes operator for managing breakglass access to the cluster.

## Features

- **Breakglass Access**: Temporary elevated access for emergency situations
- **Recurring Access**: Scheduled recurring access for maintenance and regular tasks
- **Audit Trails**: Complete logging and tracking of access requests
- **Compliance**: Built-in compliance frameworks and reporting
- **Secure**: Zero-trust principles with time-limited access
- **Privilege Escalation**: Optional mode allowing operators to grant permissions they don't hold themselves

## Quick Start

### Prerequisites

- Kubernetes cluster
- kubectl configured
- Go 1.24+ (for development)

### Installation

```bash
# Using kubectl
kubectl apply -f https://github.com/cloud-nimbus/firedoor/releases/latest/download/install.yaml

# Using Helm (coming soon)
helm install firedoor oci://ghcr.io/cloud-nimbus/firedoor/charts/firedoor
```

### Usage

```bash
# Request breakglass access
kubectl create -f - <<EOF
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: emergency-access
spec:
  # Subjects define who gets access
  subjects:
    - kind: User
      name: "alice@example.com"
    - kind: Group
      name: "devops-team"
  
  # Inline RBAC rules
  policy:
    - namespace: "production"
      rules:
        - actions: ["get", "list", "patch"]
          apiGroups: [""]
          resources: ["pods", "services"]
  
  # Approval workflow
  approval:
    required: true
  
  # Schedule defines timing and duration
  schedule:
    start: "2024-01-15T10:00:00Z"
    duration: "1h"
  
  # Required justification
  justification: "Critical production issue requiring immediate access"
EOF

### Recurring Access

For regular maintenance tasks or scheduled operations, you can create recurring breakglass access:

```bash
# Create recurring access for daily maintenance
kubectl create -f - <<EOF
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: daily-maintenance
spec:
  subjects:
    - kind: User
      name: "maintenance-team@example.com"
    - kind: Group
      name: "devops-engineers"
  
  clusterRoles:
    - "maintenance-admin"
  
  approval:
    required: false
  
  justification: "Daily system maintenance and health checks"
  
  # Schedule with recurring cron
  schedule:
    start: "2024-01-15T06:00:00Z"
    duration: "2h"
    cron: "0 6 * * 1-5"  # Every weekday at 6 AM UTC
    location: "UTC"
    maxActivations: 100  # Optional: limit total activations
EOF
```

#### Cron Schedule Format

The `schedule.cron` field uses standard 5-field cron syntax (minute hour day-of-month month day-of-week):

- `"0 9 * * 1-5"` - Weekdays at 9 AM
- `"0 0 * * *"` - Daily at midnight
- `"0 9 * * 1"` - Mondays at 9 AM
- `"0 2 1 * *"` - First day of month at 2 AM
- `"*/30 * * * *"` - Every 30 minutes

#### Recurring Access States

Recurring breakglass resources have special condition types:

- `RecurringPending`: Waiting for the next scheduled activation
- `RecurringActive`: Currently active and granting access
- `Expired`: Access has expired and will be reactivated at the next schedule

The controller automatically manages the lifecycle of recurring access, including:

- Calculating next activation times based on cron schedule
- Tracking activation counts and respecting maxActivations limits
- Managing transitions between states
- Supporting timezone-aware scheduling with the `location` field
- Providing metrics for monitoring

## Privilege Escalation Mode

By default, the Firedoor operator can only grant permissions that it holds itself. This follows the principle of least privilege and ensures security. However, in some scenarios, you may need the operator to grant elevated permissions that it doesn't currently hold.

### Enabling Privilege Escalation

To enable privilege escalation mode, set the `rbac.privilegeEscalation` flag to `true` in your Helm values:

```yaml
rbac:
  create: true
  privilegeEscalation: true  # Enable privilege escalation mode
  extraRules:
    # Your existing extra rules...
```

Or via environment variable:

```bash
export FD_CONTROLLER_PRIVILEGE_ESCALATION=true
```

### How It Works

When privilege escalation is enabled:

1. **RBAC Permissions**: The operator receives the `escalate` verb on RBAC resources, allowing it to grant permissions it doesn't hold
2. **Security Model**: The operator can create Roles/ClusterRoles with any permissions, bypassing the default Kubernetes RBAC restrictions
3. **Audit Trail**: All privilege escalation actions are logged for security monitoring

### Security Considerations

**Warning**: Privilege escalation mode bypasses Kubernetes RBAC restrictions and should be used carefully:

- **Limited Scope**: Only enable for specific use cases where elevated permissions are necessary
- **Monitoring**: Ensure comprehensive logging and monitoring of all breakglass access
- **Approval Workflow**: Always require manual approval for breakglass requests when privilege escalation is enabled
- **Time Limits**: Use short durations for elevated access
- **Regular Review**: Periodically review and audit all breakglass access

### Example Usage

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: elevated-access-example
spec:
  subjects:
    - kind: User
      name: "admin@example.com"
  
  # These permissions require privilege escalation mode
  policy:
    - rules:
        - actions: ["get", "list", "watch"]
          resources: ["nodes"]  # Cluster-scoped resource
          apiGroups: [""]
      
        - actions: ["get", "list", "watch", "create", "update", "patch", "delete"]
          resources: ["persistentvolumes"]  # Storage resources
          apiGroups: [""]
  
  approval:
    required: true  # Always require approval for elevated access
  
  schedule:
    duration: "30m"  # Short duration for elevated permissions
  
  justification: "Emergency cluster maintenance requiring node access"
```

### Best Practices

1. **Default to Disabled**: Keep privilege escalation disabled by default
2. **Enable Per Environment**: Only enable in environments where elevated access is necessary
3. **Use with Approval**: Always require manual approval when privilege escalation is enabled
4. **Monitor Usage**: Set up alerts for privilege escalation usage
5. **Regular Audits**: Review privilege escalation usage regularly
6. **Documentation**: Document why privilege escalation is needed in your environment

```

## Development

### Local Development

```bash
# Clone the repository
git clone https://github.com/cloud-nimbus/firedoor.git
cd firedoor

# Install dependencies
make install

# Run tests
make test

# Build the operator
make build

# Run locally (requires running Kubernetes cluster)
make run
```

### Using Skaffold

```bash
# Development with hot reload
skaffold dev --profile=dev

# Build for CI/CD
skaffold build --profile=ci-cd

# Deploy with telemetry
skaffold dev --profile=telemetry
```

### Version Information

```bash
# Check version information
make version

# Build with version injection
make build

# Check CLI version
./bin/firedoor version
./bin/firedoor version --output json
./bin/firedoor version --short
```

## CI/CD Pipeline

This project uses a comprehensive CI/CD pipeline with:

- **Automated Testing**: Unit tests, linting, and E2E tests
- **Semantic Versioning**: Automatic version bumping based on conventional commits
- **Multi-Architecture Builds**: Support for AMD64 and ARM64
- **Security Scanning**: Container vulnerability scanning with Trivy
- **Automated Deployment**: Development and production deployments

For detailed information about the CI/CD pipeline, see [docs/ci-cd.md](docs/ci-cd.md).

### Contributing

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for semantic versioning:

```bash
# Examples
git commit -m "feat: add OAuth2 authentication support"
git commit -m "fix: resolve race condition in controller"
git commit -m "docs: update installation instructions"
```

## Architecture

### Components

- **Controller**: Manages breakglass resources and RBAC
- **Webhook**: Validates and mutates breakglass requests
- **CLI**: Command-line interface for operators
- **Dashboard**: Web UI for managing access (coming soon)

### Security Model

- **Zero Trust**: Every access request is verified
- **Time-Limited**: All access has expiration times
- **Audited**: Complete audit trail of all actions
- **Minimal Permissions**: Principle of least privilege

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `METRICS_ADDR` | Metrics server address | `:8080` |
| `HEALTH_ADDR` | Health probe address | `:8081` |
| `LEADER_ELECT` | Enable leader election | `false` |
| `OTEL_ENABLED` | Enable OpenTelemetry | `false` |

### Configuration File

```yaml
# config.yaml
metrics:
  bind_address: ":8080"
health:
  probe_bind_address: ":8081"
manager:
  leader_elect: false
otel:
  enabled: false
  endpoint: "http://localhost:4318/v1/traces"
```

## Monitoring

### Metrics

The operator exposes Prometheus metrics on `:8080/metrics`:

- `firedoor_breakglass_requests_total`: Total breakglass requests
- `firedoor_breakglass_active`: Currently active breakglass sessions
- `firedoor_breakglass_denied_total`: Total denied requests
- `firedoor_recurring_breakglass_activation_total`: Total recurring breakglass activations
- `firedoor_recurring_breakglass_expiration_total`: Total recurring breakglass expirations
- `firedoor_recurring_breakglass_active`: Currently active recurring breakglass sessions

Namespace labels are hashed into 16 buckets (`ns_00`..`ns_0f`) to keep metric cardinality low. Use `telemetry.NamespaceBucket()` to compute the bucket for a namespace. See [docs/telemetry.md](docs/telemetry.md) for more details.

### Tracing

OpenTelemetry tracing can be enabled for observability:

The Helm chart configures the collector to listen on gRPC only. See `charts/firedoor/values.yaml` for details.

## Alerting

### Alertmanager Integration

Firedoor can send alerts to Alertmanager when breakglass access becomes active or expires. This provides real-time notifications to your team about emergency access usage.

#### Configuration

Enable Alertmanager integration in your Helm values:

```yaml
alertmanager:
  enabled: true
  url: "http://alertmanager.telemetry-system.svc.cluster.local:9093"
  timeout: 30s
  
  # Basic authentication (optional)
  basicAuth:
    username: ""
    password: ""
  
  # TLS configuration (optional)
  tls:
    insecureSkipVerify: false
    caFile: ""
    certFile: ""
    keyFile: ""
  
  # Alert configuration
  alert:
    # Labels to add to all alerts
    labels:
      team: "platform"
      component: "firedoor"
    
    # Annotations to add to all alerts
    annotations:
      runbook_url: "https://wiki.company.com/runbooks/breakglass-access"
    
    # Alert name
    alertName: "BreakglassActive"
    
    # Severity level
    severity: "warning"
    
    # Summary template
    summary: "Breakglass access is active"
    
    # Description template
    description: "A breakglass access request is currently active"
```

#### Alert Types

Firedoor sends two types of alerts:

1. **Active Alerts**: Sent when breakglass access becomes active
   - Includes justification, approved by, subjects, and expiry time
   - Alert starts when access is granted and ends when access expires

2. **Expired Alerts**: Sent when breakglass access expires
   - Includes information about the expired access
   - Used for audit and compliance purposes

#### Alert Labels and Annotations

Each alert includes:

**Labels:**

- `alertname`: The configured alert name (default: "BreakglassActive")
- `severity`: Alert severity level
- `breakglass_name`: Name of the breakglass resource
- `breakglass_namespace`: Namespace of the breakglass resource
- `status`: "active" or "expired"

**Annotations:**

- `summary`: Alert summary
- `description`: Alert description
- `justification`: The justification provided for the breakglass access
- `approved_by`: Who approved the access
- `subjects`: List of users/groups granted access
- `ticket_id`: Associated ticket ID (if provided)
- `expires_at`: When the access expires (for active alerts)
- `granted_at`: When the access was granted (for expired alerts)

#### Example Alertmanager Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: alertmanager-config
  namespace: telemetry-system
data:
  alertmanager.yaml: |
    global:
      resolve_timeout: 5m
    
    route:
      group_by: ['alertname', 'breakglass_name']
      group_wait: 10s
      group_interval: 10s
      repeat_interval: 1h
      receiver: 'breakglass-team'
      routes:
      - match:
          alertname: BreakglassActive
        receiver: 'breakglass-team'
        group_by: ['breakglass_name', 'breakglass_namespace']
        repeat_interval: 30m  # Repeat every 30 minutes while active
    
    receivers:
    - name: 'breakglass-team'
      slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK'
        channel: '#platform-alerts'
        title: '{{ template "slack.title" . }}'
        text: '{{ template "slack.text" . }}'
        send_resolved: true
    
    inhibit_rules:
    - source_match:
        alertname: BreakglassActive
        status: active
      target_match:
        alertname: BreakglassActive
        status: expired
      equal: ['breakglass_name', 'breakglass_namespace']
```

#### Metrics

Alert-related metrics are available:

- `firedoor_alerts_sent_total`: Total number of alerts sent to Alertmanager
- `firedoor_alert_send_duration_seconds`: Duration of alert send operations
- `firedoor_alert_send_errors_total`: Total number of alert send errors

See `examples/breakglass-flow/06-alertmanager-config.yaml` for a complete example configuration.

## Security

### Reporting Security Issues

Please report security vulnerabilities to <security@cloudnimbus.io>. Do not create public issues for security vulnerabilities.

### Security Scanning

All container images are automatically scanned for vulnerabilities using Trivy. Scan results are available in the GitHub Security tab.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: [docs/](docs/)
- **Issues**: [GitHub Issues](https://github.com/cloud-nimbus/firedoor/issues)
- **Discussions**: [GitHub Discussions](https://github.com/cloud-nimbus/firedoor/discussions)
- **Security**: <security@cloudnimbus.io>

## Roadmap

- [ ] Web dashboard for access management
- [ ] Integration with external identity providers
- [ ] Advanced audit and compliance reporting
- [ ] Multi-cluster support
- [ ] Custom approval workflows
- [ ] Integration with incident management systems

## Custom Resource Definitions (CRDs)

Firedoor defines the following CRD:

- **Group:** `access.cloudnimbus.io`
- **Version:** `v1alpha1`
- **Kind:** `Breakglass`

The `Breakglass` resource allows you to request and manage emergency access with enhanced RBAC modeling:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: emergency-access
spec:
  # Subjects define who gets access
  subjects:
    - kind: User
      name: "alice@example.com"
    - kind: Group
      name: "devops-team"
  
  # Use existing ClusterRoles or define inline rules
  clusterRoles: ["cluster-admin"]
  # OR
  policy:
    - namespace: "production"
      rules:
        - actions: ["get", "list", "patch"]
          apiGroups: [""]
          resources: ["pods", "services"]
  
  approval:
    required: true
  
  schedule:
    start: "2024-01-15T10:00:00Z"
    duration: "1h"
  
  justification: "Production outage troubleshooting"
  ticketID: "INC-12345"  # Optional external ticket reference
```

#### BreakglassSpec Fields

- `subjects` (required): List of users, groups, or service accounts to grant access
- `policy` (optional): Inline RBAC policy rules with optional namespace scoping
- `clusterRoles` (optional): List of existing ClusterRole names to grant
- `approval` (optional): Approval configuration (defaults to required: true)
- `schedule` (required): Timing configuration including start time, duration, and optional cron recurrence
- `justification` (required): Human-readable justification for the access request
- `ticketID` (optional): External ticket or incident identifier

#### ScheduleSpec Fields

- `start` (optional): RFC3339 timestamp when schedule becomes active
- `duration` (optional): Duration after which access is revoked
- `cron` (optional): 5-field cron expression for recurring activations
- `location` (optional): IANA timezone (defaults to UTC)
- `maxActivations` (optional): Maximum number of activations for recurring schedules

#### Example Use Cases

**One-time Emergency Access:**

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: emergency-debug
spec:
  subjects:
    - kind: User
      name: "engineer@example.com"
  policy:
    - namespace: "production"
      rules:
        - actions: ["get", "list", "watch", "exec"]
          apiGroups: [""]
          resources: ["pods"]
  schedule:
    start: "2024-01-15T14:30:00Z"
    duration: "30m"
  justification: "Debug production issue in payment service"
  ticketID: "INC-12345"
```

**Recurring Maintenance Window:**

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: weekly-maintenance
spec:
  subjects:
    - kind: Group
      name: "maintenance-team"
  clusterRoles:
    - "maintenance-admin"
  approval:
    required: false
  schedule:
    start: "2024-01-15T02:00:00Z"
    duration: "4h"
    cron: "0 2 * * 0"  # Every Sunday at 2 AM
    location: "America/New_York"
    maxActivations: 52  # Once per week for a year
  justification: "Weekly system maintenance and updates"
```

**Cluster-wide Emergency Access:**

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: cluster-emergency
spec:
  subjects:
    - kind: User
      name: "admin@example.com"
  policy:
    - rules:  # Cluster-scoped (no namespace)
        - actions: ["*"]
          apiGroups: ["*"]
          resources: ["*"]
  approval:
    required: true
  schedule:
    duration: "1h"
  justification: "Critical cluster-wide incident requiring full access"
```

See the [Helm chart](../../charts/firedoor) for installation and CRD management.
