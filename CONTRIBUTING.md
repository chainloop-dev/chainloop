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

## Development

### Chainloop Components

- `app/controlplane`
- `app/artifact-cas`
- `app/cli`

See makefiles in those directories for more information

### Using Docker Compose

You can run the core services (controlplane and CAS) and associated dependencies (postgresql) by running

```
docker compose up
```

Then, the CLI can be run by doing

```
docker compose run --rm cli
```

Note that changes made in the source code are not reflected automatically in the running services, for that you'll need to perform a restart.

```
docker compose restart -t0 controlplane
# or
docker compose restart -t0 cas
```

### Locally

Prerequisites

- postgresql

Note: You can run the prerequisites by leveraging the provided docker-compose file i.e `docker compose up postgresql`

Then each project has a `make run` target that can be used
