---
title: Use Chainloop to attest GitHub Releases
---

# Chainloop reusable workflow for GitHub Releases

You can use Chainloop to attest and collect pieces of evidence from your GitHub Releases. That includes
all assets on the GitHub Release page, such as binaries and source code. Additionally, you can attest
any other additional materials that are not part of the GitHub Release page.

## Prerequisites

There are some prerequisites to use this reusable workflow:
- You need to have an existing familiarity with Chainloop and its attestation process. Please refer to [this guide](https://docs.chainloop.dev/getting-started/attestation-crafting) to learn more.
- You need a `token` [previously generated](https://docs.chainloop.dev/reference/operator/api-tokens) by your Chainloop administrator.
- You need to have a GitHub repository with a release that you want to attest.
- You need to have a `cosign` key and passphrase to sign the attestation.
- Permissions `contents:write` if you wish to update the release notes with the Chainloop attestation link.

Optionally you need to have a workflow created in Chainloop to store the attestation, if not provided, the workflow will be created automatically.


## Where to find the reusable workflow
The reusable workflow can be found under a public repository in the Chainloop's labs GitHub organization. You can find the workflow [here](https://github.com/chainloop-dev/labs/blob/main/.github/workflows/chainloop_github_release.yml)

## How to use the reusable workflow
Create a brand-new GitHub workflow file in your repository and add the following content:

```yaml
name: Release

on:
  release:
    types: [published]

permissions:
  contents: write

jobs:
 github_release:
    name: Attest GitHub Release
    uses: chainloop-dev/labs/.github/workflows/chainloop_github_release.yml@417bad33ca08beaa785ae6a6b933406cd7b935cb
    with:
      project: "acme-team-project"
      workflow_name: "github-release-workflow"
    secrets:
      api_token: ${{ secrets.CHAINLOOP_API_TOKEN }}
      cosign_key: ${{ secrets.COSIGN_KEY }}
      cosign_password: ${{ secrets.COSING_PASSWORD }}
```

This workflow will trigger every time a new release is published in your repository. It will collect all the assets from the release page and attest them using Chainloop. The attestation will be stored in the workflow you specify in the `workflow_name` field.
There are some parameters that you need to provide:
- `workflow_name`: The name of the workflow in Chainloop where the attestation will be stored.
- `api_token`: The Chainloop API token to authenticate with the Chainloop API.
- `cosign_key`: The path to the `cosign` key file.
- `cosign_password`: The passphrase for the `cosign` key.

## How to additional materials
If you want to attest additional materials that are not part of the GitHub Release page, you can use the `additional_materials` input. For example:

```yaml
name: Release with additional materials

on:
  release:
    types: [published]

permissions:
  contents: write

jobs:
 github_release:
    name: Attest GitHub Release
    uses: chainloop-dev/labs/.github/workflows/chainloop_github_release.yml@417bad33ca08beaa785ae6a6b933406cd7b935cb
    with:
      project: "acme-team-project"
      workflow_name: "github-release-workflow"
      additional_materials: "controlplane.cyclonedx.json,ghcr.io/acme-team/acme-project:latest"
    secrets:
      api_token: ${{ secrets.CHAINLOOP_API_TOKEN }}
      cosign_key: ${{ secrets.COSIGN_KEY }}
      cosign_password: ${{ secrets.COSING_PASSWORD }}
```
A new input `additional_materials` is added to the workflow. You can provide a comma-separated list of materials that you want to attest. Chainloop will collect these materials and add them to the attestation
auto discovering their types and if cannot be inferred, they will be set as `ARTIFACT`.