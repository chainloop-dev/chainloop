---
sidebar_position: 3
title: Quickstart
---

# Quickstart

This quickstart will guide you through the process of installing the Chainloop CLI and performing your first attestation.

:::info
By default, the Chainloop CLI comes pre-configured to talk to Chainloop's platform instance, which is currently on early access. To get an account, please request access [here](https://app.chainloop.dev/request-access), and we'll get back to you shortly.

Alternatively, you can run your instance of Chainloop through our labs [docker-compose setup](https://github.com/chainloop-dev/chainloop/tree/main/devel#labs-environments) or in Kubernetes following [this guide](/guides/deployment/k8s/).
:::


1. Install CLI by running:

    ```bash
    curl -sfL https://docs.chainloop.dev/install.sh | bash -s
    ```
    Refer to [these instructions](/getting-started/installation) for more installation options.

2. Authenticate to the Control Plane:

    ```bash
    chainloop auth login
    ```

    Once logged in, your next step would be to create a Chainloop organization. Think of organizations as workspaces or namespaces. To create an organization with a random suffix, run:

    ```bash
    chainloop organization create --name quickstart-$((RANDOM % 90000 + 10000))
    # INF Organization "quickstart-10122" created!
    ```

3. Create API Token to perform the attestation process:

    To perform an attestation process, you need to provide an API Token:

    ```bash
    export CHAINLOOP_TOKEN=$(chainloop org api-token create --name test-api-token -o token)
    ```

    Chainloop API Tokens are commonly used (and required) in CI/CD scenarios. Tokens have narrower permissions, ensuring that they can only perform the operations they are granted to. More information in [API Tokens](/reference/operator/api-tokens#api-tokens).

4. Perform an attestation process:

    We are now ready to perform our first attestation, to learn more about its lifecyle refer to [this section](/getting-started/attestation-crafting#introduction)
    
    We'll start with the [initialization](/getting-started/attestation-crafting#initialization) of an attestation. The attestation process requires the name of a workflow and a project to be associated with it.

    Chainloop workflows represent any CI or process you might want to attest. Check [this doc](/getting-started/workflow-definition) for a complete explanation of Workflows and Contracts.
    You might also want to check our [contract reference](/reference/operator/contract).

    ```bash
    chainloop att init --workflow mywf --project myproject
    ```

    Once attestation is initiated, we can start [adding materials](/getting-started/attestation-crafting#adding-materials) to it. 
    In this case we are adding an OCI container image.
    Many other material types are supported, check the [updated the list](/reference/operator/contract#material-schema)

    ```bash
    chainloop att add --value ghcr.io/chainloop-dev/chainloop/control-plane:latest
    ```

    We just attested the latest version of the control-plane image as an example, remember that you can provide any material you want to attest by pointing to a local filepath too, like for example

    ```bash
    chainloop att add --value my-sbom.json
    ```

   :::info
   The piece of evidence kind were automatically detected, learn more about auto-discover [here](reference/attestations.md).
   :::

    And finally [we sign and push the attestation](/getting-started/attestation-crafting#encode-sign-and-push-attestation) to Chainloop for permanent preservation.

    ```bash
    chainloop att push
    ```

6. Operate on your data:

    At this point, we've performed our first attestation, now we can just play with the Chainloop CLI to inspect the attestation, verify it and so on. 
    
    For example, to list the workflows you can run: 

    ```bash
    # List workflow runs, so then you can do `workflow run describe --name <workflow-name>` to get more details
    chainloop workflow run ls
    ```

    for a complete list of available options and operations refer to

    ```
    chainloop --help
    ```

Great! You've successfully completed this guide. Now you are ready to dive deeper into our [Getting Started guide](/getting-started/installation)

Good luck and have fun with Chainloop! 🚀