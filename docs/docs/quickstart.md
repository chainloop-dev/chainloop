---
sidebar_position: 3
title: Quickstart
---

# Quickstart

In this quickstart, we will install Chainloop and make our first attestation:

1. Install Chainloop. This quick snippet will get you through the process:
    ```bash
    curl -sfL https://docs.chainloop.dev/install.sh | bash -s
    ```
    Refer to [these instructions](/getting-started/installation) for more installation options.

2. Authenticate to the Control Plane via OIDC by running:
    ```bash
    chainloop auth login
    ```
    This will create a personal token that you will use in operations not related to attestations.

3. Create a new Chainloop workflow.
    ```bash
    chainloop wf create --name mywf --project myproject
    ```
    Chainloop workflows represent any CI or process you might want to attest.
    Check [this doc](/getting-started/workflow-definition)
    for a complete explanation of Workflows and Contracts.
    You might also want to check our [contract reference](/reference/operator/contract).

4. Create API Token to perform the attestation process:
    ```bash
    export CHAINLOOP_TOKEN=$(chainloop org api-token create --name test-api-token -o json | jq -r ".[].jwt")
    ```
    CHAINLOOP_TOKEN environment variable is commonly used from CI/CD scenarios, where a personal token is not available.
    Tokens have narrower permissions, ensuring that they can only perform the operations they are granted to.
    More information in [API Tokens](/reference/operator/api-tokens#api-tokens).

5. Perform an attestation:
    
    First, let's [initiate the attestation](/getting-started/attestation-crafting#initialization).
    ```bash
    chainloop att init --workflow-name mywf
    ```

    Once attestation is initiated, we can start [adding materials](/getting-started/attestation-crafting#adding-materials) to it. 
    In this case we are adding an OCI container image.
    Many other material types are supported, check the [updated the list](/reference/operator/contract#material-schema)
    ```bash
    chainloop att add --value "ghcr.io/chainloop-dev/chainloop/control-plane:latest"
    ```

    And finally [we sign and push the attestation](/getting-started/attestation-crafting#encode-sign-and-push-attestation) to Chainloop for permanent preservation.
    ```bash
    chainloop att push
    ```
    Note that, in this example, we are not specifying any private key for signing.
    This will make the CLI to work in key-less mode, generating an ephemeral certificate,
    signed by Chainloop CA, to ensure the trust chain, and finally using it for the signature.