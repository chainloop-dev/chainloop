---
sidebar_position: 5
title: Operator View
---

By integrating CI/CD workloads into Chainloop control plane, the SecOps operator gain visibility on their organization automation.

## Workflows overview

You can get the list of Workflow runs and their status, including ones that have not finished yet. This is useful to define SLOs based on build times, or attestation anomalies.

```bash
$ chainloop workflow run ls
┌──────────────────────────────────────┬─────────┬────────────────┬─────────┬─────────────────────┬──────────────────────────────────────────────────────────────────┐
│ ID                                   │ PROJECT │ WORKFLOW       │ STATE   │ CREATED AT          │ RUN URL                                                          │
├──────────────────────────────────────┼─────────┼────────────────┼─────────┼─────────────────────┼──────────────────────────────────────────────────────────────────┤
│ a2f724b7-1c8c-438d-aac3-3e6b6e5be302 │ skynet  │ build-and-test │ success │ 07 Nov 22 20:45 UTC │ https://github.com/chainloop-dev/bedrock/actions/runs/3411479373 │
│ 66c89ece-9513-4ff7-9625-933340e55333 │ skynet  │ build-and-test │ success │ 07 Nov 22 15:22 UTC │ https://github.com/chainloop-dev/bedrock/actions/runs/3411479372 │
│ 4ba1bc3b-e54c-4ae5-9c31-aa44726c542f │ skynet  │ build-and-test │ failed  │ 06 Nov 22 00:14 UTC │ https://github.com/chainloop-dev/bedrock/actions/runs/3411479211 │
└──────────────────────────────────────┴─────────┴────────────────┴─────────┴─────────────────────┴──────────────────────────────────────────────────────────────────┘
```

## Attestation inspection

Get attestation information for a given workflow run

```bash
$ chainloop workflow run describe --id a2f724b7-1c8c-438d-aac3-3e6b6e5be302
┌───────────────────────────────────────────────────────────────────────────────────┐
│ Workflow                                                                          │
├────────────────┬──────────────────────────────────────────────────────────────────┤
│ ID             │ ddce7d92-cad0-4413-a5ad-99241771c537                             │
│ Name           │ build-and-test                                                   │
│ Team           │ cyberdyne core                                                   │
│ Project        │ skynet                                                           │
├────────────────┼──────────────────────────────────────────────────────────────────┤
│ Workflow Run   │                                                                  │
├────────────────┼──────────────────────────────────────────────────────────────────┤
│ ID             │ a2f724b7-1c8c-438d-aac3-3e6b6e5be302                             │
│ Initialized At │ 07 Nov 22 20:45 UTC                                              │
│ Finished At    │ 07 Nov 22 20:47 UTC                                              │
│ State          │ success                                                          │
│ Runner Type    │ Github action                                                    │
│ Runner Link    │ https://github.com/chainloop-dev/bedrock/actions/runs/3411479373 │
├────────────────┼──────────────────────────────────────────────────────────────────┤
│ Statement      │                                                                  │
├────────────────┼──────────────────────────────────────────────────────────────────┤
│ Payload Type   │ application/vnd.in-toto+json                                     │
│ Verified       │ false                                                            │
└────────────────┴──────────────────────────────────────────────────────────────────┘
┌───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Materials                                                                                                                                                                             │
├──────────────────────┬─────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ NAME                 │ TYPE            │ VALUE                                                                                                                                        │
├──────────────────────┼─────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ commit               │ STRING          │ 80e461e9b385c6986cdb8096c9dc99928943d667                                                                                                     │
│ dockerfile           │ ARTIFACT        │ Dockerfile@sha256:30cacd0de4b5142b9b3457add7acf22a48e09c3ad3c48919b3c09136e875c090                                                           │
│ rootfs               │ ARTIFACT        │ rootfs.tar.gz@sha256:b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c                                                        │
│ skynet-control-plane │ CONTAINER_IMAGE │ redacted2222.dkr.ecr.us-east-1.amazonaws.com/chainloop-control-plane@sha256:963237021c5fd0d31741a9b873e1e8af08c76459cf30e34332925510e0cb3731 │
└──────────────────────┴─────────────────┴──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
┌────────────────────────────────────────────────────────────────────┐
│ Environment Variables                                              │
├─────────────────────────┬──────────────────────────────────────────┤
│ NAME                    │ VALUE                                    │
├─────────────────────────┼──────────────────────────────────────────┤
│ GITHUB_SHA              │ 6417e226890ae1340cecc9cd9d7692445ee3338e │
│ RUNNER_NAME             │ GitHub Actions 5                         │
│ RUNNER_OS               │ Linux                                    │
│ GITHUB_ACTOR            │ migmartri                                │
│ GITHUB_REF              │ refs/tags/v0.8.10                        │
│ GITHUB_REPOSITORY       │ chainloop-dev/bedrock                    │
│ GITHUB_REPOSITORY_OWNER │ chainloop-dev                            │
│ GITHUB_RUN_ID           │ 3411479373                               │
└─────────────────────────┴──────────────────────────────────────────┘
```

To **verify the payload you can pass `--verify` and `--key` flag** pointing to the public key associated to the private one used during attestation.

```bash
$ chainloop workflow run describe --id a2f724b7-1c8c-438d-aac3-3e6b6e5be302 --verify --key consign.pub
```

:::note
Keyless verification via [sigstore Rekor](https://docs.sigstore.dev/rekor/overview/) will be implemented in a future release
:::

Raw attestation or statement can be extracted too with the **--output** flag

```json
$ chainloop workflow run describe --id a2f724b7-1c8c-438d-aac3-3e6b6e5be302 -o statement
{
   "_type": "https://in-toto.io/Statement/v0.1",
   "predicateType": "chainloop.dev/attestation/v0.1",
   "subject": [
      {
         "name": "chainloop.dev/workflow/build-and-test",
         "digest": {
            "sha256": "e9b253f90369b8ccb7fff73464f16d8e413c5cb1840bd57c42196c87235367da"
         }
      },
      {
         "name": "redacted1111.dkr.ecr.us-east-1.amazonaws.com/chainloop-control-plane",
         "digest": {
            "sha256": "963237021c5fd0d31741a9b873e1e8af08c76459cf30e34332925510e0cb3731"
         }
      }
   ],
   "predicate": {
      "buildType": "chainloop.dev/workflowrun/v0.1",
      "builder": {
         "id": "chainloop.dev/cli/0.8.9@sha256:188e2ef1c109a2ad7fdea39d1712a295c53857bcbd6f08b04d7cfdbab6d1fd30"
      },
      "env": {
         "GITHUB_ACTOR": "migmartri",
         "GITHUB_REF": "refs/tags/v0.8.10",
         "GITHUB_REPOSITORY": "chainloop-dev/bedrock",
         "GITHUB_REPOSITORY_OWNER": "chainloop-dev",
         "GITHUB_RUN_ID": "3411479373",
         "GITHUB_SHA": "6417e226890ae1340cecc9cd9d7692445ee3338e",
         "RUNNER_NAME": "GitHub Actions 5",
         "RUNNER_OS": "Linux"
      },
      "materials": [
         {
            "material": {
               "stringVal": "80e461e9b385c6986cdb8096c9dc99928943d667"
            },
            "name": "commit",
            "type": "STRING"
         },
         {
            "material": {
               "slsa": {
                  "digest": {
                     "sha256": "30cacd0de4b5142b9b3457add7acf22a48e09c3ad3c48919b3c09136e875c090"
                  },
                  "uri": "Dockerfile"
               }
            },
            "name": "dockerfile",
            "type": "ARTIFACT"
         },
         {
            "material": {
               "slsa": {
                  "digest": {
                     "sha256": "b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"
                  },
                  "uri": "rootfs.tar.gz"
               }
            },
            "name": "rootfs",
            "type": "ARTIFACT"
         },
         {
            "material": {
               "slsa": {
                  "digest": {
                     "sha256": "963237021c5fd0d31741a9b873e1e8af08c76459cf30e34332925510e0cb3731"
                  },
                  "uri": "redacted1111.dkr.ecr.us-east-1.amazonaws.com/chainloop-control-plane"
               }
            },
            "name": "skynet-control-plane",
            "type": "CONTAINER_IMAGE"
         }
      ],
      "metadata": {
         "finishedAt": "2022-11-07T21:47:03.39474301+01:00",
         "initializedAt": "2022-11-07T20:45:22.260096623Z",
         "name": "build-and-test",
         "project": "skynet",
         "team": "cyberdyne core",
         "workflowID": "ddce7d92-cad0-4413-a5ad-99241771c537",
         "workflowRunID": "a2f724b7-1c8c-438d-aac3-3e6b6e5be302"
      },
      "runnerType": "RUNNER_TYPE_GITHUB_ACTION",
      "runnerURL": "https://github.com/chainloop-dev/bedrock/actions/runs/3411479373"
   }
```

```json
$ chainloop workflow run describe --id a2f724b7-1c8c-438d-aac3-3e6b6e5be302 -o attestation
{
   "id": "97d03200-de81-46a1-a2a8-2ccec6bde932",
   "createdAt": "2022-11-07T20:47:05.687813Z",
   "envelope": {
      "payloadType": "application/vnd.in-toto+json",
      "payload": "REDACTEDOIiwicnVubmVyVVJMIjoiaHR0cHM6Ly9naXRodWIuY29tL2NoYWlubG9vcC1kZXYvYmVkcm9jay9hY3Rpb25zL3J1bnMvMzQxMTQ3OTM3MyJ9fQ==",
      "signatures": [
         {
            "keyid": "",
            "sig": "MEUCIQDMpzxxN434Vgq7QKAS7JzRRHfEFrE8elh+FZJBvOSKLwIgBh9gSreNKp3RVFNIQtrOaDrkZzhAivnfrWgEfBrzvB0="
         }
      ]
   }
}
```

## Artifacts download

You can download any artifact referenced in the statement by providing its content digest

```bash
$ chainloop artifact download -d sha256:f8a581d4bce57f792444b2230b5706a6f902fbac19a374e76f6a56f030d35cf2
INF downloading file name=rootfs.tar.gz to=/tmp/rootfs.tar.gz
INF file downloaded! path=/tmp/rootfs.tar.gz
```

## Third-Party integrations

Operators can now setup third-party integrations starting with Dependency-Track for Software Bill Of Materials (SBOMs) analysis. Read more about this capability in our [blog post](https://chainloop.dev/blog/introducing-third-party-integrations) or dive right in with [your first integration](/guides/dependency-track/) :)
