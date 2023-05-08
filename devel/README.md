# Development Environment

## Local setup

For development, Chainloop components (Control Plane, Artifact CAS and CLI) come pre-configured to talk to a set of auxiliary services (Dex, PostgresSQL and Vault) that can be run using [Docker Compose](https://docs.docker.com/compose/), so you can start contributing in no time! :)

![development environment](../docs/img/dev-env-overview.png)

### 1 - Prerequisites

To get up and running you'll need

- Golang 1.20+ toolchain
- [Docker Compose](https://docs.docker.com/compose/)
- `make` (optional)

### 2 - Run pre-required/auxiliary services

Chainloop requires some configuration to be in place such as

- PostgreSQL 14 connection from the Control plane
- Access to a secrets manager. Currently Hashicorp Vault and AWS secret manager are supported.
- Open ID connect (OIDC) single sign-on credentials.

Luckily, you can leverage the provided docker compose file that can be found in the `devel` directory to do all the setup for you.

```
cd devel
docker compose up
```

### 3 - Run Chainloop server-side components

Once you have the pre-required services up and running, you should be able to run the different Chainloop server-side components, in our case we have:

- The Control Plane [`app/controlplane`](../app/controlplane/)
- The Artifact Content Addressable Storage (CAS) Proxy [`app/artifact-cas`](../app/artifact-cas/)

These components are built using [Go](https://go.dev/), have a `Makefile` and a `make run` target for convenience.

- run controlplane `make -C app/controlplane run`
- run CAS `make -C app/cas run`

### 4 - Using the CLI pointing to the local environment

The [Command line interface (CLI)](../app/cli/) is used for both a) operate on the control plane and b) run the attestation process on your CI/CD.

You can run it by executing `go run app/cli/main.go`

4.1 - Configure the CLI to point to the local control plane and CAS services.

```
go run app/cli/main.go config save --insecure --control-plane localhost:9000 --artifact-cas localhost:9001
```

4.2 - Logging in the control-plane

You should now be ready to authenticate the CLI

> NOTE: In development a `--insecure` flag must be provided to talk to the local APIs

```
go run --insecure app/cli/main.go auth login
```

You will get redirected to the pre-configured local OIDC provider (DEX) where there are two configured users

- `sarah@chainloop.local`/`password`
- `john@chainloop.local`/`password`

Once logged in, please refer to our [Getting Started guide](https://docs.chainloop.dev/getting-started/setup) to learn how to setup an OCI registry.

