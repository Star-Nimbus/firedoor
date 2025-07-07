# Telemetry Overview

Firedoor exposes metrics and traces via the `internal/telemetry` package. To keep
Prometheus cardinality low, namespaces are mapped into 16 buckets using an FNV-1a
hash. Each bucket is labelled `ns_00` through `ns_0f`.

Use the helper function `telemetry.NamespaceBucket(<namespace>)` to compute the
bucket label for a namespace when correlating dashboards with cluster objects.

```go
fmt.Println(telemetry.NamespaceBucket("prod")) // ns_03
```

See `internal/telemetry/metrics.go` for implementation details.
