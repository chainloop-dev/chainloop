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

{{/*
Figure out the external URL for Dex service
*/}}
{{- define "chainloop.dex.external_url" -}}
{{- $service := .Values.dex.service }}
{{- $ingress := .Values.dex.ingress }}

{{- if (and $ingress $ingress.enabled $ingress.hostname) }}
{{- printf "%s://%s" (ternary "https" "http" $ingress.tls ) $ingress.hostname }}
{{- else if (and (eq $service.type "NodePort") $service.nodePorts (not (empty $service.nodePorts.http))) }}
{{- printf "http://localhost:%s" $service.nodePorts.http }}
{{- else -}}
{{- printf "http://%s:%d/dex" ( include "chainloop.dex.fullname" $ ) ( int $service.ports.http ) }}
{{- end -}}
{{- end -}}