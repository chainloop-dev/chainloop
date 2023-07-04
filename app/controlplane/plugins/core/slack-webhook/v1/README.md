# Slack Webhook Plugin

Send attestations to Slack using webhooks.

## How to use it

1. To get started, you need to register the plugin in your Chainloop organization.

```console
$ chainloop integration registered add slack-webhook --opt webhook=[webhookURL]
```

2. Attach the integration to your workflow.

```console
chainloop integration attached add --workflow $WID --integration $IID
```

## Registration Input Schema

|Field|Type|Required|Description|
|---|---|---|---|
|webhook|string (uri)|yes|URL of the slack webhook|

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/slack-webhook/v1/registration-request",
  "properties": {
    "webhook": {
      "type": "string",
      "format": "uri",
      "description": "URL of the slack webhook"
    }
  },
  "additionalProperties": false,
  "type": "object",
  "required": [
    "webhook"
  ]
}
```