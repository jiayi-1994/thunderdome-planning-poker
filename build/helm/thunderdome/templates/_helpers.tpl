{{- define "thunderdome.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 40 | trimSuffix "-" -}}
{{- end -}}

{{- define "thunderdome.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 40 | trimSuffix "-" -}}
{{- else if contains (include "thunderdome.name" .) .Release.Name -}}
{{- .Release.Name | trunc 40 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "thunderdome.name" .) | trunc 40 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "thunderdome.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | quote }}
app.kubernetes.io/name: {{ include "thunderdome.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "thunderdome.selectorLabels" -}}
app.kubernetes.io/name: {{ include "thunderdome.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "thunderdome.secretName" -}}
{{- default (printf "%s-secrets" (include "thunderdome.fullname" .)) .Values.secrets.existingSecret -}}
{{- end -}}

{{- define "thunderdome.image" -}}
{{- $proxy := .root.Values.global.imageProxy -}}
{{- if kindIs "string" .image.proxy -}}{{- $proxy = .image.proxy -}}{{- end -}}
{{- $repository := .image.repository -}}
{{- if $proxy -}}{{- $repository = printf "%s/%s" (trimSuffix "/" $proxy) $repository -}}{{- end -}}
{{- if .image.digest -}}
{{- printf "%s@%s" $repository .image.digest -}}
{{- else -}}
{{- printf "%s:%s" $repository .image.tag -}}
{{- end -}}
{{- end -}}

{{- define "thunderdome.databaseHost" -}}
{{- if .Values.postgresql.enabled -}}
{{- printf "%s-postgresql" (include "thunderdome.fullname" .) -}}
{{- else -}}
{{- required "externalDatabase.host is required when postgresql.enabled=false" .Values.externalDatabase.host -}}
{{- end -}}
{{- end -}}

{{- define "thunderdome.claimName" -}}
{{- default (printf "%s-postgresql" (include "thunderdome.fullname" .)) .Values.postgresql.persistence.existingClaim -}}
{{- end -}}
