{{- define "k8s-crondash.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "k8s-crondash.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/name: {{ include "k8s-crondash.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "k8s-crondash.selectorLabels" -}}
app.kubernetes.io/name: {{ include "k8s-crondash.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "k8s-crondash.replicaGuard" -}}
{{- if gt (int .Values.replicaCount) 1 }}
{{- fail "replicaCount must be 1 — see Trigger Safety in README" }}
{{- end }}
{{- end }}

{{- define "k8s-crondash.serviceAccountName" -}}
{{- if and (hasKey .Values "serviceAccount") (hasKey .Values.serviceAccount "name") }}
{{- .Values.serviceAccount.name }}
{{- else }}
{{- include "k8s-crondash.name" . }}
{{- end }}
{{- end }}

{{- define "k8s-crondash.resolveNamespace" -}}
{{- if eq .Values.env.namespace "-" }}
{{- .Release.Namespace }}
{{- else }}
{{- .Values.env.namespace }}
{{- end }}
{{- end }}
