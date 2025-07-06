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
app.kubernetes.io/managed-by: Helm
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
Common annotations for resource ownership
*/}}
{{- define "firedoor.annotations" -}}
meta.helm.sh/release-name: {{ .Release.Name }}
meta.helm.sh/release-namespace: {{ .Release.Namespace }}
{{- with .Values.commonAnnotations }}
{{ toYaml . }}
{{- end }}
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
{{- if .Values.openTelemetry.service }}
- --otel-service={{ .Values.openTelemetry.service }}
{{- end }}
{{- end }}
{{- if .Values.alertmanager.enabled }}
- --alertmanager-enabled=true
- --alertmanager-url={{ .Values.alertmanager.url }}
{{- if .Values.alertmanager.timeout }}
- --alertmanager-timeout={{ .Values.alertmanager.timeout }}
{{- end }}
{{- if .Values.alertmanager.basicAuth.username }}
- --alertmanager-basic-auth-username={{ .Values.alertmanager.basicAuth.username }}
{{- end }}
{{- if .Values.alertmanager.basicAuth.password }}
- --alertmanager-basic-auth-password={{ .Values.alertmanager.basicAuth.password }}
{{- end }}
{{- if .Values.alertmanager.tls.insecureSkipVerify }}
- --alertmanager-tls-insecure-skip-verify=true
{{- end }}
{{- if .Values.alertmanager.tls.caFile }}
- --alertmanager-tls-ca-file={{ .Values.alertmanager.tls.caFile }}
{{- end }}
{{- if .Values.alertmanager.tls.certFile }}
- --alertmanager-tls-cert-file={{ .Values.alertmanager.tls.certFile }}
{{- end }}
{{- if .Values.alertmanager.tls.keyFile }}
- --alertmanager-tls-key-file={{ .Values.alertmanager.tls.keyFile }}
{{- end }}
{{- if .Values.alertmanager.alert.alertName }}
- --alertmanager-alert-name={{ .Values.alertmanager.alert.alertName }}
{{- end }}
{{- if .Values.alertmanager.alert.severity }}
- --alertmanager-alert-severity={{ .Values.alertmanager.alert.severity }}
{{- end }}
{{- if .Values.alertmanager.alert.summary }}
- --alertmanager-alert-summary={{ .Values.alertmanager.alert.summary }}
{{- end }}
{{- if .Values.alertmanager.alert.description }}
- --alertmanager-alert-description={{ .Values.alertmanager.alert.description }}
{{- end }}
{{- end }}
{{- end }} 