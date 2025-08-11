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
Database host
*/}}
{{- define "ssvirt.databaseHost" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "%s-postgresql" .Release.Name }}
{{- else }}
{{- .Values.externalDatabase.host }}
{{- end }}
{{- end }}

{{/*
Database port
*/}}
{{- define "ssvirt.databasePort" -}}
{{- if .Values.postgresql.enabled }}
{{- print "5432" }}
{{- else }}
{{- printf "%v" .Values.externalDatabase.port }}
{{- end }}
{{- end }}

{{/*
Database name
*/}}
{{- define "ssvirt.databaseName" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.database }}
{{- else }}
{{- .Values.externalDatabase.database }}
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
{{- if .Values.postgresql.auth.postgresPassword }}
{{- .Values.postgresql.auth.postgresPassword }}
{{- else }}
{{- $postgresSecret := lookup "v1" "Secret" .Release.Namespace (printf "%s-postgresql" .Release.Name) }}
{{- if and $postgresSecret $postgresSecret.data (index $postgresSecret.data "postgres-password") }}
{{- index $postgresSecret.data "postgres-password" | b64dec }}
{{- else }}
{{- randAlphaNum 16 }}
{{- end }}
{{- end }}
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

{{/*
JWT Secret generation with persistence across upgrades
*/}}
{{- define "ssvirt.jwtSecret" -}}
{{- if .Values.auth.jwtSecret }}
{{- .Values.auth.jwtSecret }}
{{- else }}
{{- $existingSecret := lookup "v1" "Secret" .Release.Namespace (printf "%s-config" (include "ssvirt.fullname" .)) }}
{{- if and $existingSecret $existingSecret.data }}
{{- if hasKey $existingSecret.data "jwt-secret" }}
{{- index $existingSecret.data "jwt-secret" | b64dec }}
{{- else }}
{{- randAlphaNum 32 }}
{{- end }}
{{- else }}
{{- randAlphaNum 32 }}
{{- end }}
{{- end }}
{{- end }}

