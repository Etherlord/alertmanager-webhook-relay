{{/*
Expand the name of the chart.
*/}}
{{- define "alertmanager-webhook-relay.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "alertmanager-webhook-relay.fullname" -}}
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
{{- define "alertmanager-webhook-relay.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "alertmanager-webhook-relay.labels" -}}
helm.sh/chart: {{ include "alertmanager-webhook-relay.chart" . }}
{{ include "alertmanager-webhook-relay.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "alertmanager-webhook-relay.selectorLabels" -}}
app.kubernetes.io/name: {{ include "alertmanager-webhook-relay.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Return the Secret name to use.
*/}}
{{- define "alertmanager-webhook-relay.secretName" -}}
{{- default (include "alertmanager-webhook-relay.fullname" .) .Values.secret.existingSecret }}
{{- end }}

{{/*
Return the ConfigMap name to use.
*/}}
{{- define "alertmanager-webhook-relay.configMapName" -}}
{{- default (include "alertmanager-webhook-relay.fullname" .) .Values.configMap.existingConfigMap }}
{{- end }}

{{/*
Create the name of the service account to use.
*/}}
{{- define "alertmanager-webhook-relay.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "alertmanager-webhook-relay.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
