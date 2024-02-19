# Chainloop Module for Dagger

![dagger-tested-version](https://img.shields.io/badge/dagger%20version-v0.9.10-green)

Daggerized version of [Chainloop](https://chainloop.dev) that can be used to attest and collect pieces of evidence from your [Dagger](https://dagger.io/) pipelines.

## Prerequisites

- This module requires existing familiarity with Chainloop, and its attestation process. Please refer to [this guide](https://docs.chainloop.dev/getting-started/attestation-crafting) to learn more.
- You need a `token` (aka workflow robot account) [previously generated](https://docs.chainloop.dev/getting-started/workflow-definition#robot-account-creation) by your Chainloop administrator.

## Attestation Crafting

The [attestation process](https://docs.chainloop.dev/getting-started/attestation-crafting) starts with its initialization (`attestation-init`), then adding as many materials/pieces of evidence as needed (`attestation-add`), and finally, signing and pushing the attestation to the Chainloop control plane (`attestation-push`). 

### Init attestation ([docs](https://docs.chainloop.dev/getting-started/attestation-crafting#initialization))

Initialize an attestation using the Chainloop token stored in the `CHAINLOOP_TOKEN` environment variable.

> NOTE: `--token` can be provided only by referencing an environment variable (env:MY_VAR), not by value

```sh
dagger call -m github.com/chainloop-dev/chainloop/extras/dagger \
  --token env:CHAINLOOP_TOKEN attestation-init \
  --repository /path/to/repo \ # optional flag to automatically attest a Git repository
  --contract-revision 1 # optional flag to specify the revision of the Workflow Contract (default `latest`)
```

The result of this command will be an `attestation-id` that you will use in the next steps.

### Get the status ([docs](https://docs.chainloop.dev/getting-started/attestation-crafting#inspecting-the-crafting-status))

```sh
dagger call -m github.com/chainloop-dev/chainloop/extras/dagger \
  --token env:CHAINLOOP_TOKEN attestation-status \
  --attestation-id $ATTESTATION_ID
```

### Add pieces of evidence ([docs](https://docs.chainloop.dev/getting-started/attestation-crafting#adding-materials))

You can attest pieces of evidence by providing its material name and its value, either in the form of a path to a file (`--path`) or a raw value (`--value`).

A path to a file is required for materials derived from artifacts, such as Software Bill Of materials, or any other file-based evidence.

```sh
# Provide a material of kind artifact through its path
dagger call -m github.com/chainloop-dev/chainloop/extras/dagger \
  --token env:CHAINLOOP_TOKEN attestation-add \
  --attestation-id $ATTESTATION_ID \
  --name my-sbom \
  --path ./path/to/sbom.json

# Or one with a raw value such as a container image reference
dagger call -m github.com/chainloop-dev/chainloop/extras/dagger \
  --token env:CHAINLOOP_TOKEN attestation-add \
  --attestation-id $ATTESTATION_ID \
  --name my-container-image \
  --value ghcr.io/chainloop-dev/chainloop/control-plane
```

In some cases, you might be providing a private container image as a piece of evidence. In this case, you'll also need to provide the container registry credentials.

```sh
# Or one with a raw value such as a container image reference
dagger call -m github.com/chainloop-dev/chainloop/extras/dagger \
  --token env:CHAINLOOP_TOKEN attestation-add \
  --attestation-id $ATTESTATION_ID \
  --name my-container-image \
  --value ghcr.io/chainloop-dev/chainloop/control-plane
  --registry ghcr.io \
  --registry-username my-username \
  --registry-password MY_PAT_TOKEN
```

### Sign and push ([docs](https://docs.chainloop.dev/getting-started/attestation-crafting#encode-sign-and-push-attestation))

Sign and push the attestation using a cosign **key stored in a file** and a passphrase stored in an environment variable.

> NOTE: neither --signing-key nor --passphrase can be provided by value. You need to provide them either as a file (file:/) or an environment variable (env:/).

```sh
dagger call -m github.com/chainloop-dev/chainloop/extras/dagger \
  --token env:CHAINLOOP_TOKEN attestation-push \
  --attestation-id $ATTESTATION_ID \
  --signing-key file:/path/to/cosign.key \
  --passphrase env:COSIGN_PASSPHRASE
```

Alternatively, you can also provide the signing key in an environment variable `--signing-key env:MY_COSIGN_KEY`

### Cancel/Fail attestation

```sh
dagger call -m github.com/chainloop-dev/chainloop/extras/dagger \
  --token env:CHAINLOOP_TOKEN attestation-reset \
  --attestation-id $ATTESTATION_ID \
  --trigger failure # or --trigger cancellation
```

## Documentation

To learn more, please visit the Chainloop project's documentation website, https://docs.chainloop.dev where you will find a getting started guide, FAQ, examples, and more.

## Community / Discussion / Support

Chainloop is developed in the open and is constantly improved by our users, contributors and maintainers. Got a question, comment, or idea? Please don't hesitate to reach out via:

- GitHub [Issues](https://github.com/chainloop-dev/chainloop/issues)
- Discord [Community Server](https://discord.gg/f7atkaZact)
- Youtube [Channel](https://www.youtube.com/channel/UCISrWrPyR_AFjIQYmxAyKdg)

## Contributing

Want to get involved? Contributions are welcome.

If you are ready to jump in and test, add code, or help with documentation, please follow the instructions on
our [Contribution](CONTRIBUTING.md) page. At all times, follow our [Code of Conduct](./CODE_OF_CONDUCT.md).

See the [issue tracker](https://github.com/chainloop-dev/chainloop/issues) if you're unsure where to start, especially the [Good first issue](https://github.com/chainloop-dev/chainloop/labels/good%20first%20issue) label.