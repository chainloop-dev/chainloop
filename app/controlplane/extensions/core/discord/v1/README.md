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