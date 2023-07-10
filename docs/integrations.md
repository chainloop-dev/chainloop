# Chainloop Integrations

Operators can extend Chainloop functionality by setting up third-party integrations that operate on your attestation metadata. Integrations can range from sending a Slack message, uploading the attestation to a storage backend or sending a Software Bill Of Materials (SBOMs) to a third-party service for analysis, for example.

![FanOut Plugin](./img/fanout.png)

Below you can find the list of currently available integrations. If you can't find the integration you are looking for, feel free [to reach out](https://github.com/chainloop-dev/chainloop/issues) or [contribute your own](https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/README.md)!

## Available Integrations

| ID | Version | Description | Material Requirement |
| --- | --- | --- | --- |
| [dependency-track](https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/core/dependency-track/v1/README.md) | 1.2 | Send CycloneDX SBOMs to your Dependency-Track instance | SBOM_CYCLONEDX_JSON |
| [discord-webhook](https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/core/discord-webhook/v1/README.md) | 1.1 | Send attestations to Discord |  |
| [guac](https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/core/guac/v1/README.md) | 1.0 | Export Attestation and SBOMs metadata to a blob storage backend so guacsec/guac can consume it | SBOM_CYCLONEDX_JSON, SBOM_SPDX_JSON |
| [oci-registry](https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/core/oci-registry/v1/README.md) | 1.0 | Send attestations to a compatible OCI registry |  |
| [slack-webhook](https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/core/slack-webhook/v1/README.md) | 1.0 | Send attestations to Slack |  |
| [smtp](https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/core/smtp/v1/README.md) | 1.0 | Send emails with information about a received attestation |  |

## How to use integrations

First, you need to make sure that the integration that you are looking for is available in your Chainloop instance, to do so you will:

```sh
$ chainloop integration available list
┌──────────────────┬─────────┬──────────────────────┬───────────────────────────────────────────────────────────┐
│ ID               │ VERSION │ MATERIAL REQUIREMENT │ DESCRIPTION                                               │
├──────────────────┼─────────┼──────────────────────┼───────────────────────────────────────────────────────────┤
│ dependency-track │ 1.2     │ SBOM_CYCLONEDX_JSON  │ Send CycloneDX SBOMs to your Dependency-Track instance    │
├──────────────────┼─────────┼──────────────────────┼───────────────────────────────────────────────────────────┤
│ smtp             │ 1.0     │                      │ Send emails with information about a received attestation │
├──────────────────┼─────────┼──────────────────────┼───────────────────────────────────────────────────────────┤
│ oci-registry     │ 1.0     │                      │ Send attestations to a compatible OCI registry            │
├──────────────────┼─────────┼──────────────────────┼───────────────────────────────────────────────────────────┤
│ discord-webhook  │ 1.1     │                      │ Send attestations to Discord                              │
└──────────────────┴─────────┴──────────────────────┴───────────────────────────────────────────────────────────┘
```

Once you find your integration, i.e `oci-registry`, it's time to configure it.

Configuring an integration has two steps: 1) register it in your organization and 2) attach the registered integration to your workflows.

### Registering an integration

Registration is when a specific instance of the integration is configured on a Chainloop organization. A registered instance is then available to be attached to any workflow.

In our case, we want to register an instance of the `oci-registry` integration. To do so, we need to first figure out what configuration parameters are required by the integration. We can do so by running:

```sh
$ chainloop integration available describe --id oci-registry
┌──────────────┬─────────┬──────────────────────┬────────────────────────────────────────────────┐
│ ID           │ VERSION │ MATERIAL REQUIREMENT │ DESCRIPTION                                    │
├──────────────┼─────────┼──────────────────────┼────────────────────────────────────────────────┤
│ oci-registry │ 1.1     │                      │ Send attestations to a compatible OCI registry │
└──────────────┴─────────┴──────────────────────┴────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────┐
│ Registration inputs                                          │
├────────────┬────────┬──────────┬─────────────────────────────┤
│ FIELD      │ TYPE   │ REQUIRED │ DESCRIPTION                 │
├────────────┼────────┼──────────┼─────────────────────────────┤
│ password   │ string │ yes      │ OCI repository password     │
│ repository │ string │ yes      │ OCI repository uri and path │
│ username   │ string │ yes      │ OCI repository username     │
└────────────┴────────┴──────────┴─────────────────────────────┘
┌─────────────────────────────────────────────────────────────────────────┐
│ Attachment inputs                                                       │
├────────┬────────┬──────────┬────────────────────────────────────────────┤
│ FIELD  │ TYPE   │ REQUIRED │ DESCRIPTION                                │
├────────┼────────┼──────────┼────────────────────────────────────────────┤
│ prefix │ string │ no       │ OCI images name prefix (default chainloop) │
└────────┴────────┴──────────┴────────────────────────────────────────────┘
```

In the console output we can see a registration section that indicates that 3 parameters are required, let's go ahead and register it using our Google Artifact Registry Credentials by running:

```sh
$ chainloop integration registered add oci-registry \
    # i.e us-east1-docker.pkg.dev/my-project/chainloop-cas-devel
    --opt repository=[region]-docker.pkg.dev/[my-project]/[my-repository] \
    --opt username=_json_key \
    --opt "password=$(cat service-account.json)"
```

> Note: You can find more examples on how to configure this specific integration [here](https://github.com/chainloop-dev/chainloop/tree/main/app/controlplane/plugins/core/oci-registry/v1)

### Attaching an integration

Once the integration is registered, we can attach it to any workflow. In practice this means that attestations and material information generated in this workflow will be sent to the registered integration.

The attachment process has at least two required parameters, the `workflowID` and the registered `integrationID`. Additionally each integration might have additional to customize its behavior per-workflow. In our case, on the table above, we can see that the `oci-registry` integration has an optional parameter called `prefix` that allows you to customize the name of the image that will be pushed to the registry. 

```console 
$ chainloop integration attached add --workflow $WID --integration $IID --opt prefix=custom-prefix
```

Congratulations, you are done now! Every new attestation from this workflow will be uploaded to your OCI registry!

## FAQ

### How do I know if an integration is available?

You can use the `chainloop integration available list` command to list all the available integrations.

### How do I know what configuration parameters are required by an integration?

You can use the `chainloop integration available describe` command to list all the required configuration parameters.

### How do I know what registered integrations I have in my organization?

You can use the `chainloop integration registered list` command to list all the registered integrations.

You can also delete a registered integration by using the `chainloop integration registered delete` command.

### How do I know what attachments I have in my organization?

You can use the `chainloop integration attached list` command to list all the attachments, and detach them by using the `chainloop integration attached delete` command.

### What If I can't find the integration I am looking for?

If you can't find the integration you are looking for, feel free [to report it](https://github.com/chainloop-dev/chainloop/issues) or [contribute your own](https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/README.md)!

### I am stuck, what do I do?

If you have any questions or run into any issues, don't hesitate to reach out via our [Discord Server](https://discord.gg/f7atkaZact) or open an [Issue](https://github.com/chainloop-dev/chainloop/issues/new). We'll be happy to help.