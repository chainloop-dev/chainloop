# Chainloop Core

> Chainloop Core is under active development and you should expect breaking changes before the first stable release.
> If you are interested in contributing, please take a look at our [contributor guide](./CONTRIBUTING.md).

## What is it?

Chainloop core (Chainloop) is an open source software supply chain control plane, a single source of truth for artifacts plus a declarative attestation process.

With Chainloop, SecOps teams can declaratively state the attestation and artifacts expectations for their organization’s CI/CD workflows, while also resting assured that latest standards and best practices are put in place.

Developer teams, on the other hand, do not need to become security experts, the attestation crafting tool will guide them with guardrails and a familiar developer experience.

To learn more about the project motivation please look at [this blog post](https://docs.chainloop.dev/blog/introducing-chainloop) and see it in action in [this video](https://docs.chainloop.dev/blog/software-supply-chain-attestation-easy-way#see-it-in-action).

## Getting started

See the getting started [guide](https://docs.chainloop.dev/getting-started/installation) for detailed information on downloading and configuring the Chainloop Command Line Interface (CLI)

In a nutshell:

```bash
$ curl -sfL https://docs.chainloop.dev/install.sh | bash -s

$ chainloop auth login
```

You can also build and run Chainloop [from source](./CONTRIBUTING.md)

## How does it work?

### Compliant Single Source of Truth

Craft and store attestation metadata and artifacts via a single integration point regardless of your CI/CD provider choice.

![Chainloop Overview](./docs/img/overview-1.png)

The result is having a SLSA level 3 compliant single Source of truth for artifacts and attestation built on OSS standards such as [Sigstore](https://www.sigstore.dev/), [in-toto](https://in-toto.io/), [SLSA](https://slsa.dev) and [OCI](https://github.com/opencontainers/image-spec/blob/main/spec.md).

Chainloop also makes sure the crafting of artifacts and attestation follows **best practices and meets the requirements** declared in their associated Workflow Contract.

### Declarative, contract-based attestation

One key aspect is that in Chainloop, CI/CD integrations are declared via [**Workflow Contracts**](https://docs.chainloop.dev/getting-started/workflow-definition#workflow-contracts).

A [Workflow Contract](https://docs.chainloop.dev/reference/operator/contract) gives operators **full control over what kind of data (build info, materials) must be received as part of the attestation and the environment where these workflows must be executed at**. This enables an easy, and maintainable, way of propagating and enforcing requirements downstream to your organization.

You can think of it as an [**API for your organization's Software Supply Chain**](https://docs.chainloop.dev/reference/operator/contract) that both parties, development and SecOps teams can use to interact effectively.

![Chainloop Contracts](./docs/img/overview-3.png)

### Third-Party Integration fan-out

Operators can set up third-party integrations such as [Dependency-Track](https://docs.chainloop.dev/guides/dependency-track) for SBOM analysis or an OCI registry for storage of the received artifacts and attestation metadata.

![Chainloop Overview](./docs/img/overview-2.png)

Ops can mix and match with different integrations while **not requiring developers to make any changes on their side**!

### Role-tailored experience

Chainloop makes sure to clearly define the responsibilities, experience and functional scope of the **two main personas, Security/Operation (SecOps) and Development/Application teams**.

SecOps are the ones in charge of defining the Workflow Contracts, setting up third-party integrations, or having access to the control plane where all the Software Supply Chain Security bells and whistles are exposed.

Development teams on the other hand, just need to integrate Chainloop's jargon-free [crafting tool](https://docs.chainloop.dev/getting-started/attestation-crafting) and follow the steps via a familiar DevExp to make sure they comply with the Workflow Contract defined by the SecOps team. No need to learn in-toto, signing, SLSA, OCI, APIs, nada :)

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

## Changelog

Take a look at the list of [releases](http://github.com/chainloop-dev/chainloop/releases) to stay tuned for the latest features and changes.

## License

Chainloop is released under the Apache License, Version 2.0. Please see the [LICENSE](./LICENSE.md) file for more information.
