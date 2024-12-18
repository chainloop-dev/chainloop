# Webhook Plugin

Send attestations and SBOMs to using webhooks.

## How to use it

1. To get started, you need to register the plugin in your Chainloop organization.

```console
chainloop integration registered add webhook --name [my-registration] --opt url=[webhookURL]
```

> **Note:** The webhook URL must be accessible from the Chainloop control plane.

1. Attach the integration to your workflow.

```console
chainloop integration attached add --workflow $WID --integration $IID
```

> **Note:** You can specify the `send_attestation` and `send_sbom` options to control what is sent to the webhook. `--opt "send_attestation=false"` will disable sending attestations, and `--opt "send_sbom=true"` will enable sending SBOMs.

## Registration Input Schema

|Field|Type|Required|Description|
|---|---|---|---|
|url|string|yes|Webhook URL to send payloads to|

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/webhook/v1/registration-request",
  "properties": {
    "url": {
      "type": "string",
      "minLength": 1,
      "description": "Webhook URL to send payloads to"
    }
  },
  "additionalProperties": false,
  "type": "object",
  "required": [
    "url"
  ]
}
```

## Attachment Input Schema

|Field|Type|Required|Description|
|---|---|---|---|
|send_attestation|boolean|no|Send attestation|
|send_sbom|boolean|no|Additionally send CycloneDX or SPDX Software Bill Of Materials (SBOM)|

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/webhook/v1/attachment-request",
  "properties": {
    "send_attestation": {
      "type": "boolean",
      "description": "Send attestation",
      "default": true
    },
    "send_sbom": {
      "type": "boolean",
      "description": "Additionally send CycloneDX or SPDX Software Bill Of Materials (SBOM)",
      "default": false
    }
  },
  "additionalProperties": false,
  "type": "object"
}
```