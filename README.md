# Firedoor

Firedoor is a Kubernetes operator for managing breakglass access to the cluster.

## Features

- **Breakglass Access**: Temporary elevated access for emergency situations
- **Audit Trails**: Complete logging and tracking of access requests
- **Compliance**: Built-in compliance frameworks and reporting
- **Secure**: Zero-trust principles with time-limited access

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
  justification: "Critical production issue requiring immediate access"
  duration: "1h"
  permissions:
    - namespaces: ["production"]
      verbs: ["get", "list", "patch"]
EOF
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

### Tracing

OpenTelemetry tracing can be enabled for observability:

The Helm chart configures the collector to listen on gRPC only. See `charts/firedoor/values.yaml` for details.

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

The `Breakglass` resource allows you to request and manage emergency access. The `group` field can be used to specify a user group for access (in addition to or instead of a user):

```yaml
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: emergency-access
spec:
  group: "devops-team"
  durationMinutes: 60
  namespace: "production"
  role: "admin"
  approved: true
```

- `group`: (string) The user group to grant access to. Either `user` or `group` must be provided.
- `user`: (string) The individual user to grant access to. Either `user` or `group` must be provided.

See the [Helm chart](../../charts/firedoor) for installation and CRD management.
