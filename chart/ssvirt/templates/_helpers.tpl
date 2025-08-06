{{/*
Expand the name of the chart.
*/}}
{{- define "ssvirt.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "ssvirt.fullname" -}}
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
{{- define "ssvirt.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "ssvirt.labels" -}}
helm.sh/chart: {{ include "ssvirt.chart" . }}
{{ include "ssvirt.selectorLabels" . }}
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
{{- define "ssvirt.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ssvirt.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
API Server labels
*/}}
{{- define "ssvirt.apiServerLabels" -}}
{{ include "ssvirt.labels" . }}
app.kubernetes.io/component: api-server
{{- end }}

{{/*
API Server selector labels
*/}}
{{- define "ssvirt.apiServerSelectorLabels" -}}
{{ include "ssvirt.selectorLabels" . }}
app.kubernetes.io/component: api-server
{{- end }}

{{/*
Controller labels
*/}}
{{- define "ssvirt.controllerLabels" -}}
{{ include "ssvirt.labels" . }}
app.kubernetes.io/component: controller
{{- end }}

{{/*
Controller selector labels
*/}}
{{- define "ssvirt.controllerSelectorLabels" -}}
{{ include "ssvirt.selectorLabels" . }}
app.kubernetes.io/component: controller
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "ssvirt.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ssvirt.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the controller service account to use
*/}}
{{- define "ssvirt.controllerServiceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- printf "%s-controller" (include "ssvirt.fullname" .) }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Database URL generation (without credentials for security)
*/}}
{{- define "ssvirt.databaseUrl" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "postgresql://%s-postgresql:5432/%s" .Release.Name .Values.postgresql.auth.database }}
{{- else if .Values.database.url }}
{{- .Values.database.url }}
{{- else }}
{{- printf "postgresql://%s:%g/%s" .Values.externalDatabase.host .Values.externalDatabase.port .Values.externalDatabase.database }}
{{- end }}
{{- end }}

{{/*
Database username
*/}}
{{- define "ssvirt.databaseUsername" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.username }}
{{- else }}
{{- .Values.externalDatabase.username }}
{{- end }}
{{- end }}

{{/*
Database password
*/}}
{{- define "ssvirt.databasePassword" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.password }}
{{- else }}
{{- .Values.externalDatabase.password }}
{{- end }}
{{- end }}

{{/*
Get the PostgreSQL password secret name
*/}}
{{- define "ssvirt.databaseSecretName" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "%s-postgresql" .Release.Name }}
{{- else if .Values.externalDatabase.existingSecret }}
{{- .Values.externalDatabase.existingSecret }}
{{- else }}
{{- printf "%s-external-db" (include "ssvirt.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Get the PostgreSQL password secret key
*/}}
{{- define "ssvirt.databaseSecretPasswordKey" -}}
{{- if .Values.postgresql.enabled }}
{{- print "postgres-password" }}
{{- else if .Values.externalDatabase.existingSecret }}
{{- .Values.externalDatabase.existingSecretPasswordKey | default "password" }}
{{- else }}
{{- print "password" }}
{{- end }}
{{- end }}

{{/*
Image name template
*/}}
{{- define "ssvirt.image" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.image.registry -}}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) }}
{{- else }}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) }}
{{- end }}
{{- end }}

{{/*
Common annotations
*/}}
{{- define "ssvirt.annotations" -}}
{{- with .Values.commonAnnotations }}
{{ toYaml . }}
{{- end }}
{{- end }}
