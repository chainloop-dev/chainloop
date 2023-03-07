# Contributing to Chainloop Core

Chainloop Core maintainers welcome contributions from the community and first want to thank you for taking the time to contribute!

This project and everyone participating in it is governed by the [Code of Conduct](./CODE_OF_CONDUCT.md) before contributing. By participating, you are expected to uphold this code.

## I Have a Question

> If you want to ask a question, we assume that you have read the available [Documentation](https://docs.chainloop.dev).

Before you ask a question, it is best to search for existing [Issues](https://github.com/chainloop-dev/chainloop/issues) that might help you. In case you have found a suitable issue and still need clarification, you can write your question in this issue. It is also advisable to search the internet for answers first.

If you then still feel the need to ask a question and need clarification, we recommend the following:

- Open an [Issue](https://github.com/chainloop-dev/chainloop/issues/new).
- Provide as much context as you can about what you're running into.

We will then take care of the issue as soon as possible.

## Local development

### Pre-requisites

To run Chainloop core components locally you need

- Golang 1.20+ toolchain
- `make`

In addition to those, you'll need:

- PostgreSQL 14
- A secret manager i.e Hashicorp Vault or credentials for AWS secret manager
- Open ID connect Single-Sign-On credentials.

Luckily, these can be easily run by leveraging the provided docker compose that can be found in the `devel` directory.

```
cd devel
docker compose up
```

### Chainloop Components

- `app/controlplane`
- `app/artifact-cas`
- `app/cli`

Each of those directories have a `Makefile` with a `make run` target

i.e `make -C app/controlplane run`

### Logging in the control-plane

```
make -C app/cli -- auth login
```

this will redirect you to the pre-configured local OIDC provider (DEX) where there are two configured users

- `sarah@chainloop.local`/`password`
- `john@chainloop.local`/`password`
