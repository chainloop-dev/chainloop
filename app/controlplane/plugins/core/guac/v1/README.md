# GUAC integration

Graph for Understanding Artifact Composition ([GUAC](https://github.com/guacsec/guac)) aggregates software security metadata into a high fidelity graph database—normalizing entity identities and mapping standard relationships between them. Querying this graph can drive higher-level organizational outcomes such as audit, policy, risk management, and even developer assistance.

This integration allows Chainloop users to automatically send attestation (DSSE envelopes / in-toto statements) and CycloneDX/SPDX Software Bill Of Materials (SBOMs) to a cloud storage bucket staging area. From there, GUAC can be configured to continuously monitor and inject that data.

![GUAC integration](./img/overview.png)


## How to use it

Currently, this integration only supports [Google Cloud Storage](https://cloud.google.com/storage) (GCS) as a storage provider with more to come in the future. If you are interested in a specific provider, please [let us know](https://github.com/chainloop-dev/chainloop/issues/new)


### Chainloop setup
#### Using Google Cloud Platform

**Prerequisites**

- A Google Cloud Platform (GCP) account
- A GCP project with a GCS bucket
- A GCP service account with write access to the bucket. See [Creating and managing service accounts](https://cloud.google.com/iam/docs/creating-managing-service-accounts) for more information. 
- Once create, download the service account [JSON credentials file](https://developers.google.com/workspace/guides/create-credentials#create_credentials_for_a_service_account)


**Registration**

To get started, you need to register the plugin in your Chainloop organization.

```sh
$ chainloop integration registered add guac --opt bucket=[my-bucket-name] --opt credentials=[credentials-content] --opt provider=gcs

# Example
$ chainloop integration registered add guac --opt bucket=test-guac --opt credentials="$(cat ./service-account-devel.json)" --opt provider=gcs
```

**Attachment**

Then, in order to use the integration, you need to attach it to a workflow by providing the IDs of the workflow and integration you just registered.

```sh
$ chainloop integration attached add --workflow $WID --integration $ID
```

That's all on the Chainloop side. Now all new attestation and SBOM metadata files will get uploaded to Google Cloud Storage.


### GUAC setup

Refer to https://github.com/guacsec/guac documentation to learn how to setup GUAC to import from a GCS-based collector.


## Registration Input Schema

|Field|Type|Required|Description|
|---|---|---|---|
|bucket|string|yes|Bucket name where to store the artifacts|
|credentials|string|yes|Credentials to access the bucket|
|provider|string|no|Blob storage provider: default gcs|

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/guac/v1/registration-request",
  "properties": {
    "provider": {
      "type": "string",
      "enum": [
        "gcs"
      ],
      "minLength": 1,
      "description": "Blob storage provider: default gcs"
    },
    "bucket": {
      "type": "string",
      "minLength": 1,
      "description": "Bucket name where to store the artifacts"
    },
    "credentials": {
      "type": "string",
      "minLength": 2,
      "description": "Credentials to access the bucket"
    }
  },
  "additionalProperties": false,
  "type": "object",
  "required": [
    "bucket",
    "credentials"
  ]
}
```