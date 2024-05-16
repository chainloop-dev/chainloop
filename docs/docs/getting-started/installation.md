---
sidebar_position: 1
title: Installation
---

import Image from "@theme/IdealImage";
import Tabs from "@theme/Tabs";
import TabItem from "@theme/TabItem";

Chainloop is comprised of two main components

- A Server Side Component (Control Plane + Artifact Storage Proxy) that acts as single source of truth and management console.
- A Command Line Interface (CLI) used to both a) operate on the control plane and b) run the attestation process on your CI/CD

<Image img={require("./chainloop-parts.png")} className="light-mode-only" />
<Image img={require("./chainloop-parts-dark.png")} className="dark-mode-only" />

## Command Line Interface (CLI) installation

To **install the latest version** for macOS, Linux or Windows (using [WSL](https://learn.microsoft.com/en-us/windows/wsl/install)) just choose one of the following installation method.

<Tabs>
  <TabItem value="script" label="Installation Script" default>

```bash
curl -sfL https://docs.chainloop.dev/install.sh | bash -s
```

you can retrieve a specific version with

```bash
# You can find all the available versions at https://github.com/chainloop-dev/chainloop/releases
curl -sfL https://docs.chainloop.dev/install.sh | bash -s -- --version vx.x.x
```

and customize the install path (default to /usr/local/bin)

```bash
curl -sfL https://docs.chainloop.dev/install.sh | bash -s -- --path /my-path
```

if [`cosign`](https://docs.sigstore.dev/cosign) is present in your system, in addition to the checksum check, a signature verification will be performed. This behavior can be enforced via the `--force-verification` flag.

```bash
curl -sfL https://docs.chainloop.dev/install.sh | bash -s -- --force-verification
```

</TabItem>
<TabItem value="github" label="GitHub Release">

Refer to GitHub [releases page](https://github.com/chainloop-dev/chainloop/releases) and download the binary of your choice.

</TabItem>
<TabItem value="source" label="From Source">

```sh
git clone git@github.com:chainloop-dev/chainloop
cd chainloop && make -C app/cli build

./app/cli/bin/chainloop version
=> chainloop version v0.8.93-3-ged05b96
```

</TabItem>
</Tabs>

## Deploy Chainloop

**To run a Chainloop instance** on your Kubernetes cluster follow [these instructions](/guides/deployment/k8s).

## Configure CLI

If you [are running your own instance](/guides/deployment/k8s) of Chainloop Control Plane. You can make the CLI point to your instance by using the `chainloop config save` command.

```sh
chainloop config save \
  --control-plane my-controlplane.acme.com \
  --artifact-cas cas.acme.com
```

Another option would be to build a custom version of CLI with default endpoints' values pointing at your Chainloop instance. Please learn more about this method in [the following doc](https://github.com/chainloop-dev/chainloop/tree/main/app/cli#updating-default-values).

## Authentication

Authenticate to the Control Plane by running

```bash
$ chainloop auth login
```

That's all!

Welcome to Chainloop!
