# Firedoor Documentation

Welcome to Firedoor, a Kubernetes Breakglass Access Management System that provides secure, time-limited access to cluster resources during emergencies or maintenance windows.

## What is Firedoor?

Firedoor is a Kubernetes operator that manages "breakglass" access - temporary, elevated permissions that can be granted to users during critical situations. It provides:

- **Time-limited access**: Automatically revoke access after a specified duration
- **Recurring schedules**: Set up regular access windows for maintenance
- **RBAC integration**: Seamlessly works with Kubernetes RBAC
- **Audit trails**: Complete logging and monitoring of access events
- **Emergency protocols**: Quick access during critical situations

## Key Features

### 🔐 Secure Access Management

- Granular permission control
- Time-based access windows
- Automatic access revocation
- Integration with existing RBAC policies

### ⏰ Flexible Scheduling

- One-time access grants
- Recurring maintenance windows
- Cron-based scheduling
- Timezone support

### 📊 Monitoring & Auditing

- Comprehensive event logging
- Metrics and telemetry
- Integration with monitoring systems
- Audit trail for compliance

### 🚀 Easy Deployment

- Helm chart for easy installation
- Operator-based management
- Custom Resource Definitions (CRDs)
- GitOps ready

## Quick Start

Get started with Firedoor in minutes:

```bash
# Install Firedoor using Helm
helm repo add firedoor https://cloud-nimbus.github.io/firedoor
helm install firedoor firedoor/firedoor

# Create your first breakglass access
kubectl apply -f examples/breakglass-one-time.yaml
```

## Use Cases

### Emergency Access

When you need immediate access to fix critical issues:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: emergency-access
spec:
  schedule:
    duration: "1h"
  clusterRoles:
    - admin
```

### Maintenance Windows

Schedule regular access for system maintenance:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: maintenance-window
spec:
  schedule:
    cron: "0 2 * * 0"  # Every Sunday at 2 AM
    duration: "4h"
  clusterRoles:
    - cluster-admin
```

### Development Access

Provide developers with temporary elevated access:

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: dev-access
spec:
  schedule:
    duration: "8h"
  policy:
    - namespace: "development"
      rules:
        - apiGroups: ["*"]
          resources: ["*"]
          verbs: ["*"]
```

## Documentation Structure

- **[Getting Started](getting-started/installation.md)**: Installation and basic setup
- **[User Guide](user-guide/concepts.md)**: Core concepts and usage patterns
- **[Examples](examples/basic-usage.md)**: Real-world usage examples
- **[API Reference](api/breakglass-crd.md)**: Complete API documentation
- **[Development](development/contributing.md)**: Contributing and development guide

## Community

- **GitHub**: [Star-Nimbus/firedoor](https://github.com/Star-Nimbus/firedoor)
- **Issues**: [Report bugs or request features](https://github.com/Star-Nimbus/firedoor/issues)
- **Discussions**: [Community discussions](https://github.com/Star-Nimbus/firedoor/discussions)

## License

Firedoor is licensed under the Apache License 2.0. See the [LICENSE](https://github.com/Star-Nimbus/firedoor/blob/main/LICENSE) file for details.

---

Ready to get started? Check out our [Installation Guide](getting-started/installation.md)!
