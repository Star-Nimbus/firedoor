{{/*
Expand the name of the chart.
*/}}
{{- define "firedoor.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "firedoor.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "firedoor.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "firedoor.labels" -}}
helm.sh/chart: {{ include "firedoor.chart" . }}
{{ include "firedoor.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "firedoor.selectorLabels" -}}
app.kubernetes.io/name: {{ include "firedoor.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
control-plane: controller-manager
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "firedoor.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-controller-manager" (include "firedoor.fullname" .)) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Get the image registry
*/}}
{{- define "firedoor.imageRegistry" -}}
{{- if .Values.image.registry }}
{{- .Values.image.registry }}
{{- else }}
{{- "" }}
{{- end }}
{{- end }}

{{/*
Get the full image name
*/}}
{{- define "firedoor.image" -}}
{{- $registry := include "firedoor.imageRegistry" . }}
{{- $repository := .Values.image.repository }}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
Create the manager command arguments
*/}}
{{- define "firedoor.managerArgs" -}}
- manager
{{- if .Values.leaderElection.enabled }}
- --leader-elect
{{- end }}
{{- if .Values.healthProbe.bindAddress }}
- --health-probe-bind-address={{ .Values.healthProbe.bindAddress }}
{{- end }}
{{- if .Values.metrics.enabled }}
- --metrics-bind-address={{ .Values.metrics.bindAddress }}:{{ .Values.metrics.port }}
{{- end }}
{{- if .Values.openTelemetry.enabled }}
- --otel-enabled=true
- --otel-exporter={{ .Values.openTelemetry.exporter }}
{{- if .Values.openTelemetry.endpoint }}
- --otel-endpoint={{ .Values.openTelemetry.endpoint }}
{{- end }}
{{- if .Values.openTelemetry.headers }}
- --otel-headers={{ .Values.openTelemetry.headers | toJson }}
{{- end }}
{{- if .Values.openTelemetry.insecure }}
- --otel-insecure={{ .Values.openTelemetry.insecure }}
{{- end }}
{{- if .Values.openTelemetry.timeout }}
- --otel-timeout={{ .Values.openTelemetry.timeout }}
{{- end }}
{{- if .Values.openTelemetry.compression }}
- --otel-compression={{ .Values.openTelemetry.compression }}
{{- end }}
{{- if .Values.openTelemetry.sampler.type }}
- --otel-sampler={{ .Values.openTelemetry.sampler.type }}
{{- end }}
{{- if .Values.openTelemetry.sampler.arg }}
- --otel-sampler-arg={{ .Values.openTelemetry.sampler.arg }}
{{- end }}
{{- end }}
{{- end }} 