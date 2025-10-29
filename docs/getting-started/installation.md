# Installation

This guide will help you install Firedoor in your Kubernetes cluster.

## Prerequisites

- Kubernetes cluster (v1.20+)
- Helm 3.x
- kubectl configured to access your cluster

## Installation Methods

### Method 1: Helm Chart (Recommended)

1. **Add the Firedoor Helm repository:**

   ```bash
   helm repo add firedoor https://cloud-nimbus.github.io/firedoor
   helm repo update
   ```

2. **Install Firedoor:**

   ```bash
   helm install firedoor firedoor/firedoor
   ```

3. **Verify the installation:**

   ```bash
   kubectl get pods -n firedoor-system
   kubectl get crd breakglasses.access.cloudnimbus.io
   ```

### Method 2: Kustomize

1. **Clone the repository:**

   ```bash
   git clone https://github.com/Star-Nimbus/firedoor.git
   cd firedoor
   ```

2. **Apply the manifests:**

   ```bash
   kubectl apply -k kustomize/manager
   ```

### Method 3: Direct YAML

1. **Apply the CRD:**

   ```bash
   kubectl apply -f .generated/access.cloudnimbus.io_breakglasses.yaml
   ```

2. **Apply the operator:**

   ```bash
   kubectl apply -f kustomize/manager/manager.yaml
   ```

## Configuration

### Basic Configuration

The default configuration should work for most use cases. You can customize the installation using Helm values:

```yaml
# values.yaml
replicaCount: 1

image:
  repository: firedoor
  tag: latest
  pullPolicy: IfNotPresent

serviceAccount:
  create: true
  annotations: {}
  name: ""

rbac:
  create: true

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

nodeSelector: {}

tolerations: []

affinity: {}
```

### Advanced Configuration

For production deployments, consider these settings:

```yaml
# production-values.yaml
replicaCount: 3

image:
  tag: "v1.0.0"

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 512Mi

rbac:
  create: true
  # Add additional RBAC rules if needed

# Enable monitoring
monitoring:
  enabled: true
  serviceMonitor:
    enabled: true

# Enable telemetry
telemetry:
  enabled: true
  otel:
    endpoint: "http://otel-collector:4317"
```

## Verification

After installation, verify that Firedoor is working correctly:

1. **Check the operator pod:**

   ```bash
   kubectl get pods -l app.kubernetes.io/name=firedoor
   ```

2. **Check the CRD:**

   ```bash
   kubectl get crd breakglasses.access.cloudnimbus.io
   ```

3. **Test with a simple breakglass:**

   ```bash
   kubectl apply -f examples/breakglass-one-time.yaml
   kubectl get breakglasses
   ```

## Troubleshooting

### Common Issues

**Issue**: Pod is not starting

```bash
kubectl describe pod -l app.kubernetes.io/name=firedoor
kubectl logs -l app.kubernetes.io/name=firedoor
```

**Issue**: CRD not found

```bash
kubectl get crd | grep breakglass
# If not found, reapply the CRD
kubectl apply -f .generated/access.cloudnimbus.io_breakglasses.yaml
```

**Issue**: RBAC permissions

```bash
kubectl auth can-i create breakglasses
kubectl auth can-i create rolebindings
```

### Logs

Check the operator logs for detailed information:

```bash
kubectl logs -l app.kubernetes.io/name=firedoor -f
```

### Debug Mode

Enable debug logging by setting the log level:

```yaml
# In your values.yaml
env:
  - name: LOG_LEVEL
    value: "debug"
```

## Next Steps

- [Quick Start Guide](quick-start.md) - Create your first breakglass
- [Configuration](configuration.md) - Configure Firedoor for your needs
- [Examples](../examples/basic-usage.md) - See real-world usage examples
