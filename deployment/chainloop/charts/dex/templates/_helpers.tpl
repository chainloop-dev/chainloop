{{/*
##############################################################################
Dex helpers
##############################################################################
*/}}

{{/*
Return the proper Dex image name
*/}}
{{- define "chainloop.dex.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.dex.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper service name for Dex
*/}}
{{- define "chainloop.dex" -}}
  {{- printf "%s" (include "common.names.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{/*
Create the name of the service account to use for Dex
*/}}
{{- define "chainloop.dex.serviceAccountName" -}}
{{- if .Values.dex.serviceAccount.create -}}
    {{ default (printf "%s" (include "common.names.fullname" .)) .Values.dex.serviceAccount.name | trunc 63 | trimSuffix "-" }}
{{- else -}}
    {{ default "default" .Values.dex.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Chainloop Dex release name
*/}}
{{- define "chainloop.dex.fullname" -}}
{{- printf "%s-%s" (include "common.names.fullname" .) "dex" | trunc 63 | trimSuffix "-" -}}
{{- end -}}