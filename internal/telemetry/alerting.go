/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RecordAlertDelivery records alert delivery attempts in the operations_total metric.
// destination: e.g., "slack", "pagerduty", "email"
// result: ResultSuccess or ResultError
func RecordAlertDelivery(destination string, result Result) {
	RecordOperation(OpAlert, result, ComponentController, "n/a", "")
}

// TraceAlertDelivery emits a span or event for alert delivery attempts.
// ctx: context for tracing
// tracer: the tracer to use
// destination: e.g., "slack", "pagerduty", "email"
// alertType: e.g., "breakglass_active", "breakglass_expired"
// result: ResultSuccess or ResultError
// duration: time taken to send the alert (seconds)
func TraceAlertDelivery(ctx context.Context, tracer trace.Tracer, destination, alertType string, result Result, duration time.Duration) {
	ctx, span := tracer.Start(ctx, "notify."+destination,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("alert.type", alertType),
			attribute.String("result", string(result)),
			attribute.Float64("duration_sec", duration.Seconds()),
		),
	)
	if result == ResultError {
		span.SetStatus(codes.Error, "alert delivery failed")
	} else {
		span.SetStatus(codes.Ok, "")
	}
	defer span.End()
}
