{{- define "pull-secrets-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "pull-secrets-operator.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- include "pull-secrets-operator.name" . -}}
{{- end -}}
{{- end -}}

{{- define "pull-secrets-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "pull-secrets-operator.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "pull-secrets-operator.labels" -}}
app.kubernetes.io/name: {{ include "pull-secrets-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | quote }}
{{- end -}}

{{- define "pull-secrets-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pull-secrets-operator.name" . }}
app.kubernetes.io/component: manager
{{- end -}}
