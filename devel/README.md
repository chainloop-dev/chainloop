## Local development

### Pre-requisites

To run Chainloop core components locally you need

- Golang 1.20+ toolchain
- `make`

In addition to those, you'll need:

- PostgreSQL 14
- Access to a credentials manager. Currently Hashicorp Vault and AWS secret manager are supported.
- Open ID connect (OIDC) single sign-on credentials.

Luckily, these can be easily run by leveraging the provided docker compose that can be found in the `devel` directory.

```
cd devel
docker compose up
```

### Chainloop Components

Once you have the pre-required service up and running, you should be able to run the different Chainloop components, in our case we have three.

- The Control Plane (`app/controlplane`)
- The Artifact Content Addressable Storage (CAS) Proxy (`app/artifact-cas`)
- The Command line interface (CLI) that is used for both a) operate on the control plane and b) run the attestation process on your CI/CD (`app/cli`)

These three components are built in Golang and have a `Makefile` and a `make run` target for convenience.

i.e `make -C app/controlplane run`

### Logging in the control-plane

Once you have the Controlplane and the Artifact API running, you can get started by authenticating using the CLI

```
make -C app/cli -- auth login
```

You will get redirected to the pre-configured local OIDC provider (DEX) where there are two configured users

- `sarah@chainloop.local`/`password`
- `john@chainloop.local`/`password`
