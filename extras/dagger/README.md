# Chainloop Module for Dagger

![dagger-tested-version](https://img.shields.io/badge/dagger%20version-v0.10.0-green)

Daggerized version of [Chainloop](https://docs.chainloop.dev) that can be used to attest and collect pieces of evidence from your [Dagger](https://dagger.io/) pipelines.

## Prerequisites

- This module requires existing familiarity with Chainloop and its attestation process. Please refer to [this guide](https://docs.chainloop.dev/getting-started/attestation-crafting) to learn more.
 You need an `API Token` [previously generated](https://docs.chainloop.dev/getting-started/workflow-definition#api-token-creation) by your Chainloop administrator.

## Attestation Crafting

The [attestation process](https://docs.chainloop.dev/getting-started/attestation-crafting) starts with its initialization (`init`) or `resume`, then adding as many materials/pieces of evidence as needed (`add-raw-evidence` or `add-file-evidence`), and finally, signing and pushing the attestation to the Chainloop control plane (`push`).

You can invoke this module in two ways: either from the Dagger CLI `dagger call ...` or from your own Dagger pipeline by importing this module as a dependency.

### Using the Chainloop module in your Dagger pipeline

To use Chainloop in your module, first, you need to add it as a dependency.

```sh
dagger install github.com/chainloop-dev/chainloop
```

Once done, you'll have access to the Chainloop client via `dag.Chainloop()` and start the attestation process with Init().

You can find a full example of how to integrate attestation crafting in your `Go` pipeline [here](https://github.com/chainloop-dev/integration-demo/blob/main/chainloop-demo/dagger/src/main.go)

### Using the Dagger CLI

The [attestation process](https://docs.chainloop.dev/getting-started/attestation-crafting) starts with its initialization (`init`) or `resume`, then adding as many materials/pieces of evidence as needed (`add-raw-evidence` or `add-file-evidence`), and finally, signing and pushing the attestation to the Chainloop control plane (`push`).

This module is designed to support function chaining, so after initializing the attestation, you can chain the subcommands to add pieces of evidence and push the attestation. For example

```sh
dagger call -m github.com/chainloop-dev/chainloop \
  # Initialize the command
  init --token env:CHAINLOOP_TOKEN \
  # we chain subcommands after the initialization
  # add a raw evidence
  add-raw-evidence --name my-evidence --value "my-value" \
  # and push the result
  push --key file:/path/to/cosign.key --passphrase env:COSIGN_PASSPHRASE
```

If the attestation process end-to-end is not completed in one go, you can store the attestation-id after init and resume the attestation process using the `resume` method at any time down the line.

```sh
# Initialize but this time we store the attestation-id
ATTESTATION_ID=$(dagger call -m github.com/chainloop-dev/chainloop  init --token env:CHAINLOOP_TOKEN attestation-id)


# and we use it to resume the attestation process
dagger call -m github.com/chainloop-dev/chainloop \
  resume --token env:CHAINLOOP_TOKEN --attestation-id $ATTESTATION_ID \
  # we chain subcommands after the initialization
  ....
```

#### 1 - Init attestation ([docs](https://docs.chainloop.dev/getting-started/attestation-crafting#initialization))

Initialize an attestation using the Chainloop token stored in the `CHAINLOOP_TOKEN` environment variable.

> NOTE: `--token` can be provided only by referencing an environment variable (env:MY_VAR), not by value

```sh
# Initialize the attestation and get its ID
dagger call -m github.com/chainloop-dev/chainloop \
  init \
  --token env:CHAINLOOP_TOKEN \
  --repository /path/to/repo \ # optional flag to automatically attest a Git repository
  --contract-revision 1 \ # optional flag to specify the revision of the Workflow Contract (default `latest`)
  --workflow-name the-name-of-the-workflow
```

#### 2 - Get the status ([docs](https://docs.chainloop.dev/getting-started/attestation-crafting#inspecting-the-crafting-status))

Resuming a previous attestation

```sh
dagger call -m github.com/chainloop-dev/chainloop \
  resume --token env:CHAINLOOP_TOKEN --attestation-id $ATTESTATION_ID \
  status
```

or chaining the command right after initialization

```sh
dagger call -m github.com/chainloop-dev/chainloop \
  init --token env:CHAINLOOP_TOKEN \
  status
```

#### 3 - Add pieces of evidence ([docs](https://docs.chainloop.dev/getting-started/attestation-crafting#adding-materials))

You can attest pieces of evidence by providing its material name and its value, either in the form of a path to a file (`--path`) or a raw value (`--value`).

A path to a file is required for materials derived from artifacts, such as Software Bill Of materials, or any other file-based evidence.

```sh
# Provide a material of kind artifact through its path
# Remember, we first resume the attestation or chain the commands
dagger call -m github.com/chainloop-dev/chainloop \
  resume --token env:CHAINLOOP_TOKEN --attestation-id $ATTESTATION_ID \
  add-file-evidence --name my-sbom --path ./path/to/sbom.json
```

```sh
# Or one with a raw value such as a container image reference
dagger call -m github.com/chainloop-dev/chainloop \
  resume --token env:CHAINLOOP_TOKEN --attestation-id $ATTESTATION_ID \
  add-raw-evidence --name my-container-image --value ghcr.io/chainloop-dev/chainloop/control-plane
```

If you're attesting materials that don't belong to the target contract, you can allow Chainloop to figure out its type and if discovered, it will be added to the attestation.
In order to do that, don't pass the `--name`, just provide the path to the file.

```sh
# Provide a material only through its path
dagger call -m github.com/chainloop-dev/chainloop \
  resume --token env:CHAINLOOP_TOKEN --attestation-id $ATTESTATION_ID \
  add-file-evidence --path ./path/to/sbom.json
```

In some cases, you might be providing a private container image as a piece of evidence. In this case, you'll also need to preload the container registry credentials.

```sh
dagger call -m github.com/chainloop-dev/chainloop \
  resume --token env:CHAINLOOP_TOKEN --attestation-id $ATTESTATION_ID \
  # Now load the registry credentials
  with-registry --address ghcr.io --username my-username --password MY_PAT_TOKEN \
  # And perform the attestation of the private container image
  add-raw-evidence --name my-container-image --value ghcr.io/chainloop-dev/chainloop/control-plane
```

#### 4 - Sign and push attestation ([docs](https://docs.chainloop.dev/getting-started/attestation-crafting#encode-sign-and-push-attestation))

Sign and push the attestation using a cosign **key stored in a file** and a passphrase stored in an environment variable.

> NOTE: neither --signing-key nor --passphrase can be provided by value. You need to provide them either as a file (file:/) or an environment variable (env:/).

```sh
dagger call -m github.com/chainloop-dev/chainloop \
  resume --token env:CHAINLOOP_TOKEN --attestation-id $ATTESTATION_ID \
  push --key file:/path/to/cosign.key --passphrase env:COSIGN_PASSPHRASE
```

Alternatively, you can also provide the signing key in an environment variable `--key env:MY_COSIGN_KEY`

#### 5 - Cancel/mark attestation as failed

```sh
dagger call -m github.com/chainloop-dev/chainloop \
  resume --token env:CHAINLOOP_TOKEN --attestation-id $ATTESTATION_ID \
  mark-failed --reason "Something went wrong"
```

or cancel the attestation process

```sh
dagger call -m github.com/chainloop-dev/chainloop \
  resume --token env:CHAINLOOP_TOKEN --attestation-id $ATTESTATION_ID \
  mark-canceled --reason "nothing to see here"
```

## Documentation

To learn more, please visit the Chainloop project's documentation website, https://docs.chainloop.dev where you will find a getting started guide, FAQ, examples, and more.

## Community / Discussion / Support

Chainloop is developed in the open and is constantly improved by our users, contributors and maintainers. Got a question, comment, or idea? Please don't hesitate to reach out via:

- GitHub [Issues](https://github.com/chainloop-dev/chainloop/issues)
- [Slack](https://join.slack.com/t/chainloop-community/shared_invite/zt-2k34dvx3r-u85uGP_KiLC6ic5Wy4aRnQ)
- Youtube [Channel](https://www.youtube.com/channel/UCISrWrPyR_AFjIQYmxAyKdg)
