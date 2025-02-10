---
sidebar_position: 4
title: Attestation Crafting
---

<iframe
  width="100%"
  height="500"
  src="https://www.youtube-nocookie.com/embed/Q_0dlBqKtIU" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe>

## Introduction

In the previous section, we created a workflow definition, a contract and an API Token in the control plane. Next, we'll perform an attestation crafting example using Chainloop.

The lifecycle of a crafting process has the following stages: `init`, `add`, `push` or `reset`. As you can see, it mimics the workflow of a commonly used version control tool, and this is not by coincidence. Chainloop wants to make sure that the tooling feels familiar to developers and that no security jargon leaks into this stage of the process. For a developer, creating an attestation must be as simple as initializing it, adding materials to it and pushing it.

A brief description of the different stages

#### attestation init

During this stage, the crafting tool will contact the control plane to

- Signal the intent of starting an attestation.
- Retrieve or create the associated workflow contract
- If the contract has a specified runner context type, check that we are compliant with it.
- Initialize environment variables, explicitly stated in the contract and other contextual information.

#### attestation add

Add the **materials required by the contract** and any other additional pieces of evidence, i.e artifact, OCI image ref, SBOM.

The `add` command knows how to handle each kind of material transparently to the user.

For example

- ARTIFACT kinds will be uploaded to your artifact registry and referenced by their content digest.
- CONTAINER_IMAGE kinds will be resolved to obtain their repository digests using the local authentication keychain.
- SBOM_CYCLONEDX_JSON will validate the right SBOM format and upload it to the artifact registry.

For a complete list of available material types, see the [reference](/reference/operator/contract#material-schema).

#### attestation push

This stage will take the current crafting state, validate that it has all the required materials and

- Create a signed, in-toto attestation envelope.
- Push it to the control plane for storage

:::note
Chainloop leverages Cosign for signing and verifying, so it supports any of its key providers.
Currently, local file-based `cosign private key` and GCP, AWS, Azure and Hashicorp Vault KMS are supported.

In future releases this will not be needed since we will rely on keyless signing and verification.
:::

#### attestation reset

By using the `reset` command we can indicate to the control plane that something went wrong or we want to abort the attestation process.

#### attestation status

See the state of the current crafting process.

## Crafting our first attestation locally

To create an attestation two things are required, the Chainloop crafting tool and an [API Token](/reference/operator/api-tokens).

The crafting tool is currently bundled within Chainloop command line tool. To install it just follow the [installation](installation) instructions.

The API Token was created during the [previous step](./workflow-definition#api-token-creation) and it's required during all the stages of the crafting process. It can be provided via the `--token` flag or the `$CHAINLOOP_TOKEN` environment variable.

```bash
$ export CHAINLOOP_TOKEN=deadbeef
```

### Initialization

#### Options

`chainloop attestation init` supports the following options

- `--token` token provided by the SecOps team. Alternatively, you can set the `CHAINLOOP_TOKEN` environment variable (required).
- `--workflow` name of the workflow to run the attestation (required). **It will create the workflow if it doesn't exist**
- `--project` name of the project of the workflow  (required).
- `--dry-run`; do not store the attestation in the Control plane, and do not fail if the runner context or required env variables can not be resolved. Useful for development (default: `false`).

:::tip
If the workflow with name `--workflow` in project `--project` doesn't exist, it will be created with the default contract (or with the contract specified by `--contract`).
:::

To initialize a new crafting process just run `attestation init` and the system will retrieve the latest version (if no specific revision is set via the `--revision` flag) of the contract.

```bash
$ chainloop attestation init --workflow build-and-test --project skynet
```

```
INF Attestation initialized! now you can check its status or add materials to it
┌───────────────────┬──────────────────────────────────────┐
│ Initialized At    │ 02 Nov 22 10:04 UTC                  │
├───────────────────┼──────────────────────────────────────┤
│ Workflow          │ 2d289d33-8241-47b7-9ea2-8bd8b7c126f8 │
│ Name              │ build-and-test                       │
│ Team              │ cyberdyne core                       │
│ Project           │ skynet                               │
│ Contract Revision │ 2                                    │
└───────────────────┴──────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────────┐
│ Materials                                                           │
├──────────────────────┬─────────────────┬─────┬──────────┬───────────┤
│ NAME                 │ TYPE            │ SET │ REQUIRED │ IS OUTPUT │
├──────────────────────┼─────────────────┼─────┼──────────┼───────────┤
│ skynet-control-plane │ CONTAINER_IMAGE │ No  │ Yes      │ x         │
│ rootfs               │ ARTIFACT        │ No  │ Yes      │           │
│ dockerfile           │ ARTIFACT        │ No  │ No       │           │
│ build-ref            │ STRING          │ No  │ Yes      │           │
│ skynet-sbom          │ SBOM_CYCLONEDX_JSON│No│ Yes      │           │
└──────────────────────┴─────────────────┴─────┴──────────┴───────────┘
┌───────────────────────────────┐
│ Env Variables                 │
├───────────────────┬───────────┤
│ GITHUB_REF        │ NOT FOUND │
│ GITHUB_RUN_ID     │ NOT FOUND │
│ GITHUB_REPOSITORY │ NOT FOUND │
└───────────────────┴───────────┘
```

As you can see, we have some work to do to complete this attestation, we have three required materials not set, let's do that next.

### Adding Materials

```bash
# Add container image
$ chainloop attestation add --name skynet-control-plane --value  ***.dkr.ecr.us-east-1.amazonaws.com/skynet-control-plane:v0.7.6
INF material added to attestation

# Add rootfs artifact
$ chainloop attestation add --name rootfs --value rootfs.tar.gz
rootfs.tar.gz@sha256:f8a581d4bce57f792444b2230b5706a6f902fbac19a374e76f6a56f030d35cf2 ... done! [7B in 0s; 34B/s]
INF material added to attestation

# Add build-ref artifact
$ chainloop attestation add --name build-ref --value 80e461e9b385c6986cdb8096c9dc99928943d667

# Add Software Bill Of Materials
$ chainloop attestation add --name skynet-sbom --value sbom.cyclonedx.json
```

:::tip
There is also the option of leaving Chainloop CLI to figure out the material type when adding a piece of evidence in an attestation, learn more about auto-discover [here](../reference/attestations.md#auto-discovery-of-pieces-of-evidence).
:::

### Inspecting the crafting status

If we check the status of the attestation we'll see that the three required materials have been added

```bash
$ chainloop attestation status --full
┌───────────────────┬──────────────────────────────────────┐
│ Initialized At    │ 02 Nov 22 10:04 UTC                  │
├───────────────────┼──────────────────────────────────────┤
│ Workflow          │ 2d289d33-8241-47b7-9ea2-8bd8b7c126f8 │
│ Name              │ build-and-test                       │
│ Team              │ cyberdyne core                       │
│ Project           │ skynet                               │
│ Contract Revision │ 2                                    │
└───────────────────┴──────────────────────────────────────┘
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Materials                                                                                                                                                                                                          │
├──────────────────────┬─────────────────┬─────┬──────────┬───────────┬──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ NAME                 │ TYPE            │ SET │ REQUIRED │ IS OUTPUT │ VALUE                                                                                                                                        │
├──────────────────────┼─────────────────┼─────┼──────────┼───────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ skynet-control-plane │ CONTAINER_IMAGE │ Yes │ Yes      │ x         │ **.dkr.ecr.us-east-1.amazonaws.com/skynet-control-plane@sha256:963237021c5fd0d31741a9b873e1e8af08c76459cf30e34332925510e0cb3731              │
│ rootfs               │ ARTIFACT        │ Yes │ Yes      │           │ rootfs.tar.gz@sha256:f8a581d4bce57f792444b2230b5706a6f902fbac19a374e76f6a56f030d35cf2                                                        │
│ dockerfile           │ ARTIFACT        │ No  │ No       │           │                                                                                                                                              │
│ build-ref            │ STRING          │ Yes │ Yes      │           │ 80e461e9b385c6986cdb8096c9dc99928943d667                                                                                                     │
│ skynet-sbom          │ SBOM_CYCLONEDX_ │ Yes │ Yes      │           │ deadbeefddaaae-redacted                                                                                                                      │
└──────────────────────┴─────────────────┴─────┴──────────┴───────────┴──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
┌───────────────────────────────┐
│ Env Variables                 │
├───────────────────┬───────────┤
│ GITHUB_REF        │ NOT FOUND │
│ GITHUB_RUN_ID     │ NOT FOUND │
│ GITHUB_REPOSITORY │ NOT FOUND │
└───────────────────┴───────────┘

┌────────────────────────────────────────────────────────────────────┐
│ Runner context                                                     │
├─────────────────────────┬──────────────────────────────────────────┤
│ GITHUB_SHA              │ a206e709cc21b1bf8e262604a23f9d0fc51a293a │
│ RUNNER_NAME             │ Hosted Agent                             │
│ RUNNER_OS               │ Linux                                    │
│ GITHUB_ACTOR            │ migmartri                                │
│ GITHUB_REF              │ refs/tags/v0.8.9                         │
│ GITHUB_REPOSITORY       │ chainloop-dev/bedrock                    │
│ GITHUB_REPOSITORY_OWNER │ chainloop-dev                            │
│ GITHUB_RUN_ID           │ 3410079758                               │
└─────────────────────────┴──────────────────────────────────────────┘
```

### Encode, sign and push attestation

:::note
Chainloop leverages Cosign for signing and verifying, so it supports any of its key providers.
Currently, local file-based `cosign private key` and GCP, AWS, Azure and Hashicorp Vault KMS are supported.
In future releases this will not be needed since we will rely on keyless signing and verification.
:::

Since all the required materials have been attached, a **signed in-toto statement can now be generated and sent for storage**. 

```bash
# Sign and push using a local private key
$ export CHAINLOOP_SIGNING_PASSWORD="private key passphrase"
$ chainloop attestation push --key cosign-private.key
```

```bash
# or, as additional example, using KMS
$ chainloop attestation push --key awskms:///arn:aws:kms:us-east-1:1234567890:key/12345678-a843-43e1-8c5b-1234567890
```

The resulting attestation will be rendered, signed and pushed to the control plane.

```json
INF Attestation pushed!
{
   "payloadType": "application/vnd.in-toto+json",
   "payload": "eyJfdHlwZSI6Imh0dHBzOi8vaW4tdG90by5pby9TdGF0ZW1lbnQvdjAuMSIsInByZWRpY2F0ZVR5cGUiOiJjaGFpbmxvb3AuZGV2L2F0dGVzdGF0aW9uL3YwLjEiLCJzdWJqZWN0IjpbeyJuYW1lIjoiY2hhaW5sb29wLmRldi93b3JrZmxvdy9idWlsZC1hbmQtdGVzdCIsImRpZ2VzdCI6eyJzaGEyNTYiOiI3ODFkZDExMWQ3NjIzNGMxMmExYzY3NmMxM2ZhZWEwYzQ5NzZmMDRhZGQ4YzhhNzY3MTQxNjQ2ZDIyMzVjNmU4In19LHsibmFtZSI6IjUyOTM0NzEyNjE2NS5ka3IuZWNyLnVzLWVhc3QtMS5hbWF6b25hd3MuY29tL2NoYWlubG9vcC1jb250cm9sLXBsYW5lIiwiZGlnZXN0Ijp****",
   "signatures": [
      {
         "keyid": "",
         "sig": "MEUCIGtMsHEwJr9oN4PcE/X9cE84BFnGM3WuQ4bXXAc/15VPAiEAqpScGVSINSmJoida/FNWKnYt64xcSE3sEcMkJwFv/H0="
      }
   ]
}
```

## CI integration

Native CI/CD runner integrations (i.e Jenkins plugin, GitHub action) are under development but the process stated above can be implemented in any CI pipeline by just using the Chainloop CLI.

See below an example of Chainloop integrated with a Github Action release job that leverages [goreleaser](https://goreleaser.com/) for building container images and binaries and AWS ECR for storage.

In that example we enable attestation to meet the requirements of [this contract](https://github.com/chainloop-dev/chainloop/blob/main/docs/examples/contracts/skynet/contract.yaml)

:::info
Remember to remove the `--dry-run` flag during intialization
:::

```yaml title=".github/workflows/release.yaml"
name: Release
on:
  push:
    tags:
      - "v*.*.*"
jobs:
  release:
    env:
      # highlight-start
      # Version of Chainloop to install
      CHAINLOOP_VERSION: 0.91.1
      # Used by the CLI to authenticate with the control plane
      CHAINLOOP_TOKEN: ${{ secrets.CHAINLOOP_WF_RELEASE }}
      # The name of the workflow registered in Chainloop control plane
      CHAINLOOP_WORKFLOW_NAME: build-and-test
      CHAINLOOP_PROJECT: skynet
      # highlight-end
    name: "Release CLI and container images"
    runs-on: ubuntu-latest
    permissions:
      id-token: write # required to use OIDC and retrieve AWS credentials
      contents: write # required for goreleaser
    steps:
      # Cosign is used to verify the Chainloop binary (optional)
      - name: Install Cosign
        uses: sigstore/cosign-installer@v2.5.0

      # highlight-start
      - name: Install Chainloop
        run: |
          curl -sfL https://docs.chainloop.dev/install.sh | bash -s -- --version v${{ env.CHAINLOOP_VERSION }}
      # highlight-end
      - name: Checkout
        uses: actions/checkout@v4

      # highlight-start
      - name: Initialize Attestation
        run: |
          chainloop attestation init --workflow $CHAINLOOP_WORKFLOW_NAME --project skynet
      # highlight-end
      - name: Set up Go
        uses: actions/setup-go@v3
        with:
          go-version: "1.23.6"

      - name: Configure AWS credentials to push container images
        uses: aws-actions/configure-aws-credentials@v1
        with:
          role-to-assume: arn:aws:iam::[REDACTED]
          aws-region: us-east-1

      - name: Login to Amazon ECR
        uses: aws-actions/amazon-ecr-login@v1

      - name: Run GoReleaser
        id: release
        uses: goreleaser/goreleaser-action@v3
        with:
          distribution: goreleaser
          version: latest
          args: release --rm-dist
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          COSIGN_PASSWORD: ${{ secrets.COSIGN_PASSWORD }}

      # Generate SBOM using syft in cycloneDX format
      - uses: anchore/sbom-action@v0
        with:
          image: ****.dkr.ecr.us-east-1.amazonaws.com/container-image:${{ github.ref_name }}
          format: cyclonedx-json
          output-file: /tmp/skynet.cyclonedx.json

      # highlight-start
      - name: Add Attestation Artifacts
        run: |
          # Add binaries created by goreleaser
          chainloop attestation add --name [binary-name] --value [binary-path]

          # Created container image
          chainloop attestation add --name control-plane-image --value ****.dkr.ecr.us-east-1.amazonaws.com/container-image:${{ github.ref_name }}

          # This is just an example of adding a key/val material type
          # Alternatively, GITHUB_SHA could have been added to the contract env variables allowList
          chainloop attestation add --name build-ref --value ${GITHUB_SHA}

          # Attach SBOM
          chainloop attestation add --name skynet-sbom --value /tmp/skynet.cyclonedx.json

      - name: Finish and Record Attestation
        if: ${{ success() }}
        run: |
          chainloop attestation status --full
          # Note that these commands are using CHAINLOOP_TOKEN env variable to authenticate
          chainloop attestation push --key env://CHAINLOOP_SIGNING_KEY
        env:
          CHAINLOOP_SIGNING_KEY: ${{ secrets.COSIGN_KEY }}
          CHAINLOOP_SIGNING_PASSWORD: ${{ secrets.COSIGN_PASSWORD }}

      - name: Mark attestation as failed
        if: ${{ failure() }}
        run: |
          chainloop attestation reset

      - name: Mark attestation as cancelled
        if: ${{ cancelled() }}
        run: |
          chainloop attestation reset --trigger cancellation
      # highlight-end
```

You can find other CI pipeline examples [here](https://github.com/chainloop-dev/chainloop/blob/main/docs/examples/ci-workflows).
