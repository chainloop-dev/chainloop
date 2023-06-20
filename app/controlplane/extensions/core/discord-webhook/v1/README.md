# Discord Webhook Extension

Send attestations to Discord using webhooks.
## How to use it

1. To get started, you need to register the extension in your Chainloop organization.

```console
$ chainloop integration registered add discord-webhook --opt webhook=[webhookURL]
```

optionally you can specify a custom username

```console
$ chainloop integration registered add discord-webhook --opt webhook=[webhookURL] --opt username=[username]
```

2. Attach the integration to your workflow.

```console
chainloop integration attached add --workflow $WID --integration $IID
```

## Registration Input Schema

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/chainloop-dev/chainloop/app/controlplane/extensions/core/discord-webhook/v1/registration-request",
  "properties": {
    "webhook": {
      "type": "string",
      "format": "uri",
      "description": "URL of the discord webhook"
    },
    "username": {
      "type": "string",
      "minLength": 1,
      "description": "Override the default username of the webhook"
    }
  },
  "additionalProperties": false,
  "type": "object",
  "required": [
    "webhook"
  ]
}
```

## Attachment Input Schema

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/chainloop-dev/chainloop/app/controlplane/extensions/core/discord-webhook/v1/registration-request",
  "properties": {
    "webhook": {
      "type": "string",
      "format": "uri",
      "description": "URL of the discord webhook"
    },
    "username": {
      "type": "string",
      "minLength": 1,
      "description": "Override the default username of the webhook"
    }
  },
  "additionalProperties": false,
  "type": "object",
  "required": [
    "webhook"
  ]
}
```