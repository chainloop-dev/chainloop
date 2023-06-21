# OCI registry extension

Send attestations to a compatible OCI repository.

## How to use it

1. To get started, you need to register the extension in your Chainloop organization.

```console
$ chainloop integration registered add oci-registry --opt repository=[repo] --opt username=[username] --opt password=[password]
```

2. When attaching the integration to your workflow, you have the option to specify an image name prefix:

```console
chainloop integration attached add --workflow $WID --integration $IID --opt prefix=custom-prefix
```

## Examples different providers

See below a non-exhaustive list of examples for different OCI registry providers known to work well with this extension.

### Google Artifact Registry

Using json-based service account https://console.cloud.google.com/iam-admin/serviceaccounts

```console
$ chainloop integration registered add oci-registry \
    # i.e us-east1-docker.pkg.dev/my-project/chainloop-cas-devel
    --opt repository=[region]-docker.pkg.dev/[my-project]/[my-repository] \
    --opt username=_json_key \
    --opt "password=$(cat service-account.json)"
```

### GitHub packages

Using personal access token with write:packages permissions https://github.com/settings/tokens

```console
$ chainloop integration registered add oci-registry \
    # i.e ghcr.io/chainloop-dev/chainloop-cas
    --opt repository=ghcr.io/[username or org]/[my-repository] \
    --opt username=[username] \
    --opt password=[personal access token]
```

### DockerHub

Create a personal access token at https://hub.docker.com/settings/security

```console
$ chainloop integration registered add oci-registry \
    --opt repository=index.docker.io/[username] \
    --opt username=[username] \
    --opt password=[personal access token]
```

### AWS Container Registry

Not supported at the moment



## Registration Input Schema

|Field|Type|Required|Description|
|---|---|---|---|
|password|string|yes|OCI repository password|
|repository|string|yes|OCI repository uri and path|
|username|string|yes|OCI repository username|

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/oci-registry/v1/registration-request",
  "properties": {
    "repository": {
      "type": "string",
      "minLength": 1,
      "description": "OCI repository uri and path"
    },
    "username": {
      "type": "string",
      "minLength": 1,
      "description": "OCI repository username"
    },
    "password": {
      "type": "string",
      "minLength": 1,
      "description": "OCI repository password"
    }
  },
  "additionalProperties": false,
  "type": "object",
  "required": [
    "repository",
    "username",
    "password"
  ]
}
```

## Attachment Input Schema

|Field|Type|Required|Description|
|---|---|---|---|
|prefix|string|no|OCI images name prefix (default chainloop)|

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/oci-registry/v1/attachment-request",
  "properties": {
    "prefix": {
      "type": "string",
      "minLength": 1,
      "description": "OCI images name prefix (default chainloop)"
    }
  },
  "additionalProperties": false,
  "type": "object"
}
```