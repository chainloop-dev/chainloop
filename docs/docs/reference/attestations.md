---
sidebar_position: 1
title: Attestation Process, advanced features
---

The basics of the attestation process is described in our [getting started guide](/getting-started/attestation-crafting). This guide, instead will focus in some advance features guide

## Project and Versions

During the attestation process, you can provide the name of the project the workflow belongs to and optionally a version.

For example, the following line will perform an attestation associated with the project `myproject` and no version `1.0.0 (pre-release)`.

```sh
$ chainloop att init --workflow mywf --project myproject
```

Optionally you can provide a version either explicitly with the `--version` flag or using a `.chainloop.yml` file (more on that below).

```sh
$ chainloop att init --workflow mywf --project myproject --version 1.0.0

┌───────────────────┬──────────────────────────────────────┐
│ Initialized At    │ 05 Nov 24 14:36 UTC                  │
├───────────────────┼──────────────────────────────────────┤
│ Attestation ID    │ 0c6c780c-7a95-4e18-9f94-b27c5ae7de6f │
│ Organization      │ miguel                               │
│ Name              │ mywf                                 │
│ Project           │ myproject                            │
│ Version           │ 1.0.0 (prerelease)                   │
│ Contract Revision │ 1                                    │
└───────────────────┴──────────────────────────────────────┘
```

As you might have noticed in the table above, the version is `1.0.0 (prerelease)`, this is because, by default, Chainloop considers all versions pre-release until they are explicitly promoted with the `--release` flag.

```
$ chainloop att init --workflow mywf --project myproject --version 1.0.0 --release
```

Once the attestation is successfully crafted and pushed, the version will be promoted to `1.0.0`.

```sh
$ chainloop att push
```

```sh
$ chainloop wf run ls
┌──────────────────────────────────────┬─────────────────────────────────┬───────────────────────┬─────────────┬─────────────────────┬─────────────────┬────────┐
│ ID                                   │ WORKFLOW                        │ VERSION               │ PRERELEASE  │ STATE               │ CREATED AT      │ RUNNER │
├──────────────────────────────────────┼─────────────────────────────────┼───────────────────────┼─────────────┼─────────────────────┼─────────────────┼────────┤
│ e293221a-5e28-4ffa-af02-0a07be908866 │ myproject/mywf                  │ 1.0.0                 │ success     │ 05 Nov 24 14:38 UTC │ Unspecified     │        │
```

This gives you control on the lifecycle of your project versions, deciding when a version is ready to be promoted to production.

### Automatically load the version

An alternative to provide the --version flag in each attestation is to use a `.chainloop.yml` file in your repo.

```yaml
# your-project/.chainloop.yml
projectVersion: v0.1.0 # example version
```

The CLI, during `attestation init` will traverse up the directory tree and load the version from the `.chainloop.yml` file automatically if it exists.

## Contract-less pieces of evidence
A Workflow Contract specifies the necessary content that a workflow must include in its attestation. For instance, it might mandate the inclusion of the URI@digest of the generated container image, the container root filesystem used during the build, and the Software Bill of Materials for that container image. These pieces of evidence must be associated with a specific material type. Operators define what must be included in the [contracts](operator/contract.mdx), and developers need to understand and comply with these requirements.

This has been the case up until now, with the introduction of contract-less pieces of evidences and auto-discovery, we ease the job of operators and developers when working with attestations.

Not all pieces of evidences need to to be registered as a material on the contract, you can add as many pieces of evidences as you like by adding 
its value and a new flag `--kind`, which determines that type of material you’re attesting, example:

```bash
$ chainloop attestation init --workflow wf-test --project core
INF Attestation initialized! now you can check its status or add materials to it
┌───────────────────┬──────────────────────────────────────┐
│ Initialized At    │ 21 May 24 07:23 UTC                  │
├───────────────────┼──────────────────────────────────────┤
│ Attestation ID    │ 6e9d2e76-fdc0-4493-a344-4cf44a2b7bf2 │
│ Name              │ wf-test                              │
│ Team              │ founding                             │
│ Project           │ core                                 │
│ Contract Revision │ 3                                    │
└───────────────────┴──────────────────────────────────────┘
┌────────────────────────┐
│ Materials              │
├───────────┬────────────┤
│ Name      │ one-file   │
│ Type      │ ARTIFACT   │
│ Set       │ No         │
│ Required  │ Yes        │
│ Is output │ Yes        │
├───────────┼────────────┤
│ Name      │ other-file │
│ Type      │ EVIDENCE   │
│ Set       │ No         │
│ Required  │ Yes        │
│ Is output │ Yes        │
└───────────┴────────────┘
```
There are two expected materials of kind `ARTIFACT` and `EVIDENCE`. With the changes we can perform the following:

```bash
$ chainloop attestation add --value controlplane.cyclonedx.json  --kind SBOM_CYCLONEDX_JSON
INF material added to attestation
```
We are explicitly saying which is the kind (`SBOM_CYCLONEDX_JSON`) we want to apply to material that is not found on the contract. After fulfilling the required materials, 
the ones stated on the contract, we can perform the attestation as usual:

```bash
$ chainloop attestation push
INF push completed
┌───────────────────┬──────────────────────────────────────┐
│ Initialized At    │ 21 May 24 06:37 UTC                  │
├───────────────────┼──────────────────────────────────────┤
│ Attestation ID    │ 57b8dc22-d84d-40c3-a04c-8948231b3134 │
│ Name              │ wf-test                              │
│ Team              │ founding                             │
│ Project           │ core                                 │
│ Contract Revision │ 3                                    │
└───────────────────┴──────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────────────────────────┐
│ Materials                                                                           │
├───────────┬─────────────────────────────────────────────────────────────────────────┤
│ Name      │ one-file                                                                │
│ Type      │ ARTIFACT                                                                │
│ Set       │ Yes                                                                     │
│ Required  │ Yes                                                                     │
│ Is output │ Yes                                                                     │
│ Value     │ go.mod                                                                  │
│ Digest    │ sha256:29773f085c46a33efcb6cdb185f6ec30ce1c4ca708b860708cd055b70488ef4d │
├───────────┼─────────────────────────────────────────────────────────────────────────┤
│ Name      │ other-file                                                              │
│ Type      │ EVIDENCE                                                                │
│ Set       │ Yes                                                                     │
│ Required  │ Yes                                                                     │
│ Is output │ Yes                                                                     │
│ Value     │ LICENSE.md                                                              │
│ Digest    │ sha256:c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4 │
├───────────┼─────────────────────────────────────────────────────────────────────────┤
│ Name      │ material-1716273574284654000                                            │
│ Type      │ SBOM_CYCLONEDX_JSON                                                     │
│ Set       │ Yes                                                                     │
│ Required  │ No                                                                      │
│ Is output │ Yes                                                                     │
│ Value     │ controlplane.cyclonedx.json                                             │
│ Digest    │ sha256:a6bc29d7a2d8d9f6df12a86ee4c86c58189d77bb6ded9487330c39f46ee00d9a │
└───────────┴─────────────────────────────────────────────────────────────────────────┘
Attestation Digest: sha256:61832d846b870d01647a384c3df49e3c75fd87f45821c9295d97ab91d5bae198
```

## Auto-discovery of pieces of evidence
In top of contract-less pieces of evidences, we have introduced auto-discovery. Auto-discovery it’s a way of inspecting the incoming piece of evidence and try to match it with at least of of the available type of [materials](operator/contract.mdx#material-schema) of Chainloop. Please note this is a best effort and the prediction might fail and matching it with the wrong type of material, defaulting in `ARTIFACT`.

In order to let the auto-discovery work, don’t set `--name` nor `--kind`, let the CLI work it our for you.

Example of usage. Given the following contract:

```bash
$ chainloop attestation init --workflow wf-test --project core
INF Attestation initialized! now you can check its status or add materials to it
┌───────────────────┬──────────────────────────────────────┐
│ Initialized At    │ 22 May 24 13:38 UTC                  │
├───────────────────┼──────────────────────────────────────┤
│ Attestation ID    │ 583553ef-d051-4c41-aec4-a4cdd725bf89 │
│ Name              │ wf-test                              │
│ Team              │ founding                             │
│ Project           │ core                                 │
│ Contract Revision │ 3                                    │
└───────────────────┴──────────────────────────────────────┘
┌────────────────────────┐
│ Materials              │
├───────────┬────────────┤
│ Name      │ one-file   │
│ Type      │ ARTIFACT   │
│ Set       │ No         │
│ Required  │ Yes        │
│ Is output │ Yes        │
├───────────┼────────────┤
│ Name      │ other-file │
│ Type      │ EVIDENCE   │
│ Set       │ No         │
│ Required  │ Yes        │
│ Is output │ Yes        │
└───────────┴────────────┘
```
Let's add the required materials:

```bash
$ chainloop attestation add --value go.mod --name one-file
INF material added to attestation

$ chainloop attestation add --value LICENSE.md --name other-file
INF material added to attestation
```
And finally let's try to discover one material without specifying its type:

```bash
$ chainloop attestation add --value controlplane.cyclonedx.json
INF material kind detected kind=SBOM_CYCLONEDX_JSON
INF material added to attestation
```
As a result we can see how it's added to the result:

```bash
$ chainloop attestation push
INF push completed
┌───────────────────┬──────────────────────────────────────┐
│ Initialized At    │ 22 May 24 13:38 UTC                  │
├───────────────────┼──────────────────────────────────────┤
│ Attestation ID    │ 583553ef-d051-4c41-aec4-a4cdd725bf89 │
│ Name              │ wf-test                              │
│ Team              │ founding                             │
│ Project           │ core                                 │
│ Contract Revision │ 3                                    │
└───────────────────┴──────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────────────────────────┐
│ Materials                                                                           │
├───────────┬─────────────────────────────────────────────────────────────────────────┤
│ Name      │ one-file                                                                │
│ Type      │ ARTIFACT                                                                │
│ Set       │ Yes                                                                     │
│ Required  │ Yes                                                                     │
│ Is output │ Yes                                                                     │
│ Value     │ go.mod                                                                  │
│ Digest    │ sha256:29773f085c46a33efcb6cdb185f6ec30ce1c4ca708b860708cd055b70488ef4d │
├───────────┼─────────────────────────────────────────────────────────────────────────┤
│ Name      │ other-file                                                              │
│ Type      │ EVIDENCE                                                                │
│ Set       │ Yes                                                                     │
│ Required  │ Yes                                                                     │
│ Is output │ Yes                                                                     │
│ Value     │ LICENSE.md                                                              │
│ Digest    │ sha256:c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4 │
├───────────┼─────────────────────────────────────────────────────────────────────────┤
│ Name      │ material-1716385111238449000                                            │
│ Type      │ SBOM_CYCLONEDX_JSON                                                     │
│ Set       │ Yes                                                                     │
│ Required  │ No                                                                      │
│ Value     │ controlplane.cyclonedx.json                                             │
│ Digest    │ sha256:a6bc29d7a2d8d9f6df12a86ee4c86c58189d77bb6ded9487330c39f46ee00d9a │
└───────────┴─────────────────────────────────────────────────────────────────────────┘
Attestation Digest: sha256:8a0b3a9db0372fdf571dbe85c8a9b5202f473ca97e9dbcdf77c3f9b423ea3b9c
```

Contract-less and auto-discovery and two features that walk hand by hand. They compose a new way of adding pieces of evidences to an existing contract in a frictionless way. You can see it in action in our [quickstart](../quickstart.md) guide.

## Remote State

By default, the attestation process state is stored locally. But this setup is not suitable when running a multi-step attestation process in a stateless environment, like our Dagger module, or when you want to leverage CI multi-job parallelism or similar.

For that, we implemented attestation remote state. Simply put, instead of the attestation CLI being in charge of maintaining the state during the attestation, this can be delegated to the server and retrieved at any time by providing an “attestation-id.”


```sh
# You can enable the feature by providing the --remote-state flag
# and it will return an attestation-id
$ chainloop attestation init --name my-workflow --project my-project --remote-state

# Then you can add any piece of evidence by providing the attestation-id
$ chainloop attestation add --value cyberdyne.cyclonedx.sbom --attestation-id deadbeef

# And finally craft the attestation, sign-it and push it
$ chainloop attestation push --attestation-id deadbeef
```

With this optional feature, as long as you have the attestation-id, you can add any piece of evidence to the attestation from anywhere.