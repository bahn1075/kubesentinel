{{/* 차트 이름 */}}
{{- define "kubesentinel-ai.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* 전체 이름 (release-chart) */}}
{{- define "kubesentinel-ai.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/* 공통 라벨 */}}
{{- define "kubesentinel-ai.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "kubesentinel-ai.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/* 셀렉터 라벨 */}}
{{- define "kubesentinel-ai.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubesentinel-ai.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/* ServiceAccount 이름 */}}
{{- define "kubesentinel-ai.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "kubesentinel-ai.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/* Secret 이름 (existingSecret 우선) */}}
{{- define "kubesentinel-ai.secretName" -}}
{{- if .Values.secret.existingSecret -}}
{{- .Values.secret.existingSecret -}}
{{- else -}}
{{- include "kubesentinel-ai.fullname" . -}}
{{- end -}}
{{- end -}}

{{/* 이미지 태그 (미지정 시 appVersion) */}}
{{- define "kubesentinel-ai.imageTag" -}}
{{- default .Chart.AppVersion .Values.image.tag -}}
{{- end -}}

{{/* ── Frontend ── */}}
{{- define "kubesentinel-ai.frontend.fullname" -}}
{{- printf "%s-frontend" (include "kubesentinel-ai.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kubesentinel-ai.frontend.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubesentinel-ai.name" . }}-frontend
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "kubesentinel-ai.frontend.labels" -}}
{{ include "kubesentinel-ai.frontend.selectorLabels" . }}
app.kubernetes.io/component: frontend
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "kubesentinel-ai.frontend.imageTag" -}}
{{- default .Chart.AppVersion .Values.frontend.image.tag -}}
{{- end -}}
