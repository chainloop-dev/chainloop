# Chainloop Module for Dagger

![dagger-min-version](https://img.shields.io/badge/dagger%20version-v0.9.8-yellow)

Daggerized version of [Chainloop](https://chainloop.dev) that can be used to attest and collect pieces of evidence from your [Dagger](https://dagger.io/) pipelines.

## Prerequisites

- This module requires existing familiarity with Chainloop, and its attestation process. Please refer to [this guide](https://docs.chainloop.dev/getting-started/attestation-crafting) to learn more.
- You need a `token` (aka workflow robot account) [previously generated](https://docs.chainloop.dev/getting-started/workflow-definition#robot-account-creation) by your Chainloop administrator. 

## Attestation Crafting

The [attestation process](https://docs.chainloop.dev/getting-started/attestation-crafting) starts with its initialization (`attestation-init`), then adding as many materials/pieces of evidence as needed (`attestation-add`), and finally, signing and pushing the attestation to the Chainloop control plane (`attestation-push`). 

### Init attestation ([docs](https://docs.chainloop.dev/getting-started/attestation-crafting#initialization))

```sh
dagger call -m github.com/chainloop-dev/chainloop/extras/dagger \
  --token $CHAINLOOP_TOKEN attestation-init
```

The result of this command will be an `attestation-id` that you will use in the next steps.

### Get the status ([docs](https://docs.chainloop.dev/getting-started/attestation-crafting#inspecting-the-crafting-status))

```sh
dagger call -m github.com/chainloop-dev/chainloop/extras/dagger \
  --token $CHAINLOOP_TOKEN attestation-status \
  --attestation-id $ATTESTATION_ID
```

### Add pieces of evidence ([docs](https://docs.chainloop.dev/getting-started/attestation-crafting#adding-materials))

```sh
dagger call -m github.com/chainloop-dev/chainloop/extras/dagger \
  --token $CHAINLOOP_TOKEN attestation-add \
  --attestation-id $ATTESTATION_ID \
  --name [MATERIAL NAME] \
  --value [MATERIAL_VALUE]    
```

### Sign and push ([docs](https://docs.chainloop.dev/getting-started/attestation-crafting#encode-sign-and-push-attestation))

```sh
dagger call -m github.com/chainloop-dev/chainloop/extras/dagger \
  --token $CHAINLOOP_TOKEN attestation-push \
  --attestation-id $ATTESTATION_ID \
  --signing-key [path/to/cosign.key] \
  --passphrase [cosign-passphrase]
```

### Cancel/Fail attestation

```sh
dagger call -m github.com/chainloop-dev/chainloop/extras/dagger \
  --token $CHAINLOOP_TOKEN attestation-reset \
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