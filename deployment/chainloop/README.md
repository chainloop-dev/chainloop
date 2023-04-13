# Chainloop Helm Chart

[Chainloop](https://github.com/chainloop-dev/chainloop) is an open-source software supply chain control plane, a single source of truth for artifacts plus a declarative attestation crafting process.

## Introduction

This chart bootstraps a [Chainloop](https://github.com/chainloop-dev/chainloop) deployment on a [Kubernetes](https://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- PV provisioner support in the underlying infrastructure
- ReadWriteMany volumes for deployment scaling

## Installing the Chart

TODO

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```console
helm delete my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Parameters

### Common parameters

| Name                    | Description                                                                                                                                                            | Value   |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- |
| `kubeVersion`           | Override Kubernetes version                                                                                                                                            | `""`    |
| `development`           | Deploys Chainloop pre-configured FOR DEVELOPMENT ONLY. It includes a Vault instance in development mode and pre-configured authentication certificates and passphrases | `false` |
| `GKEMonitoring.enabled` | Enable GKE podMonitoring (monitoring.googleapis.com/v1) to scrape the controlplane and CAS prometheus endpoints                                                        | `false` |

### Secrets Backend

| Name                                            | Description                                                          | Value       |
| ----------------------------------------------- | -------------------------------------------------------------------- | ----------- |
| `secrets_backend.backend`                       | Secrets backend type ("vault" or "aws_secret_manager")               | `vault`     |
| `secrets_backend.secret_prefix`                 | Prefix that will be pre-pended to all secrets in the storage backend | `chainloop` |
| `secrets_backend.vault.address`                 | Vault address                                                        |             |
| `secrets_backend.vault.token`                   | Vault authentication token                                           |             |
| `secrets_backend.aws_secret_manager.access_key` | AWS Access KEY ID                                                    |             |
| `secrets_backend.aws_secret_manager.secret_key` | AWS Secret Key                                                       |             |
| `secrets_backend.aws_secret_manager.region`     | AWS Secret Manager Region                                            |             |

### Authentication

| Name               | Description                                                            | Value |
| ------------------ | ---------------------------------------------------------------------- | ----- |
| `casJWTPrivateKey` | ECDSA (ES512) private Key used for Controlplane <-> CAS Authentication | `""`  |
| `casJWTPublicKey`  | ECDSA (ES512) public key                                               | `""`  |

### Control Plane

| Name                            | Description                                                                         | Value                                           |
| ------------------------------- | ----------------------------------------------------------------------------------- | ----------------------------------------------- |
| `controlplane.replicaCount`     | Number of replicas                                                                  | `2`                                             |
| `controlplane.image.repository` | FQDN uri for the image                                                              | `ghcr.io/chainloop-dev/chainloop/control-plane` |
| `controlplane.image.tag`        | Image tag (immutable tags are recommended). If no set chart.appVersion will be used |                                                 |

### Control Plane Database

| Name                                     | Description                                                | Value   |
| ---------------------------------------- | ---------------------------------------------------------- | ------- |
| `controlplane.externalDatabase.host`     | Database host                                              | `""`    |
| `controlplane.externalDatabase.port`     | Database port number                                       | `5432`  |
| `controlplane.externalDatabase.user`     | Non-root username                                          | `""`    |
| `controlplane.externalDatabase.database` | Database name                                              | `""`    |
| `controlplane.externalDatabase.password` | Password for the non-root username                         | `""`    |
| `controlplane.sqlProxy.enabled`          | Enable sidecar to connect to DB via Google Cloud SQL proxy | `false` |
| `controlplane.sqlProxy.connectionName`   | Google Cloud SQL connection name                           | `""`    |
| `controlplane.sqlProxy.resources`        | Sidecar container resources                                | `{}`    |

### Control Plane Authentication

| Name                                    | Description                                                                                            | Value   |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------ | ------- |
| `controlplane.auth.passphrase`          | Passphrase used to sign the Auth Tokens generated by the controlplane. Leave empty for auto-generation | `""`    |
| `controlplane.auth.oidc.url`            | Full authentication path, it should match the issuer URL of the Identity provider (IDp)                | `""`    |
| `controlplane.auth.oidc.client_id`      | OIDC IDp client_id                                                                                     | `""`    |
| `controlplane.auth.oidc.client_secret`  | OIDC IDp client_secret                                                                                 | `""`    |
| `controlplane.auth.redirect_url_scheme` | Schema that will be used during authentication                                                         | `https` |

### Control Plane Networking

| Name                                     | Description                                                                                                                      | Value                    |
| ---------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------ |
| `controlplane.service.type`              | Service type                                                                                                                     | `ClusterIP`              |
| `controlplane.service.port`              | Service port                                                                                                                     | `80`                     |
| `controlplane.service.targetPort`        | Service target Port                                                                                                              | `http`                   |
| `controlplane.service.nodePorts.http`    | Node port for HTTP. NOTE: choose port between <30000-32767>                                                                      |                          |
| `controlplane.serviceAPI.type`           | Service type                                                                                                                     | `ClusterIP`              |
| `controlplane.serviceAPI.port`           | Service port                                                                                                                     | `80`                     |
| `controlplane.serviceAPI.targetPort`     | Service target Port                                                                                                              | `grpc`                   |
| `controlplane.serviceAPI.annotations`    | Service annotations                                                                                                              |                          |
| `controlplane.serviceAPI.nodePorts.http` | Node port for HTTP. NOTE: choose port between <30000-32767>                                                                      |                          |
| `controlplane.ingress.enabled`           | Enable ingress record generation for %%MAIN_CONTAINER_NAME%%                                                                     | `false`                  |
| `controlplane.ingress.pathType`          | Ingress path type                                                                                                                | `ImplementationSpecific` |
| `controlplane.ingress.hostname`          | Default host for the ingress record                                                                                              | `cp.dev.local`           |
| `controlplane.ingress.ingressClassName`  | IngressClass that will be be used to implement the Ingress (Kubernetes 1.18+)                                                    | `""`                     |
| `controlplane.ingress.path`              | Default path for the ingress record                                                                                              | `/`                      |
| `controlplane.ingress.annotations`       | Additional annotations for the Ingress resource. To enable certificate autogeneration, place here your cert-manager annotations. | `{}`                     |
| `controlplane.ingress.tls`               | Enable TLS configuration for the host defined at `controlplane.ingress.hostname` parameter                                       | `false`                  |
| `controlplane.ingress.selfSigned`        | Create a TLS secret for this ingress record using self-signed certificates generated by Helm                                     | `false`                  |
| `controlplane.ingress.extraHosts`        | An array with additional hostname(s) to be covered with the ingress record                                                       | `[]`                     |
| `controlplane.ingress.extraPaths`        | An array with additional arbitrary paths that may need to be added to the ingress under the main host                            | `[]`                     |
| `controlplane.ingress.extraTls`          | TLS configuration for additional hostname(s) to be covered with this ingress record                                              | `[]`                     |
| `controlplane.ingress.secrets`           | Custom TLS certificates as secrets                                                                                               | `[]`                     |
| `controlplane.ingressAPI.enabled`        | Resource enabled                                                                                                                 | `false`                  |
| `controlplane.ingressAPI.className`      | IngressClass that will be be used                                                                                                | `""`                     |
| `controlplane.ingressAPI.annotations`    | Annotations                                                                                                                      | `{}`                     |
| `controlplane.ingressAPI.hosts`          | HTTP hosts                                                                                                                       | `[]`                     |
| `controlplane.ingressAPI.tls`            | TLS configuration ref: https://kubernetes.io/docs/concepts/services-networking/ingress/#tls                                      | `[]`                     |

### Controlplane Misc

| Name                                                         | Description                   | Value   |
| ------------------------------------------------------------ | ----------------------------- | ------- |
| `controlplane.resources.limits`                              | Container resource limits     | `{}`    |
| `controlplane.resources.requests`                            | Container resource requests   | `{}`    |
| `controlplane.autoscaling.enabled`                           | Enable deployment autoscaling | `false` |
| `controlplane.autoscaling.minReplicas`                       | Minimum number of replicas    | `1`     |
| `controlplane.autoscaling.maxReplicas`                       | Maximum number of replicas    | `100`   |
| `controlplane.autoscaling.targetCPUUtilizationPercentage`    | Target CPU percentage         | `80`    |
| `controlplane.autoscaling.targetMemoryUtilizationPercentage` | Target CPU memory             | `80`    |

### Artifact Content Addressable (CAS) API

| Name                   | Description                                                                         | Value                                          |
| ---------------------- | ----------------------------------------------------------------------------------- | ---------------------------------------------- |
| `cas.replicaCount`     | Number of replicas                                                                  | `2`                                            |
| `cas.image.repository` | FQDN uri for the image                                                              | `ghcr.io/chainloop-dev/chainloop/artifact-cas` |
| `cas.image.tag`        | Image tag (immutable tags are recommended). If no set chart.appVersion will be used |                                                |

### CAS Networking

| Name                            | Description                                                                                 | Value       |
| ------------------------------- | ------------------------------------------------------------------------------------------- | ----------- |
| `cas.serviceAPI.type`           | Service type                                                                                | `ClusterIP` |
| `cas.serviceAPI.port`           | Service port                                                                                | `80`        |
| `cas.serviceAPI.targetPort`     | Service target Port                                                                         | `grpc`      |
| `cas.serviceAPI.annotations`    | Service annotations                                                                         |             |
| `cas.serviceAPI.nodePorts.http` | Node port for HTTP. NOTE: choose port between <30000-32767>                                 |             |
| `cas.ingressAPI.enabled`        | Resource enabled                                                                            | `false`     |
| `cas.ingressAPI.className`      | IngressClass that will be be used                                                           | `""`        |
| `cas.ingressAPI.annotations`    | Annotations                                                                                 | `{}`        |
| `cas.ingressAPI.hosts`          | HTTP hosts                                                                                  | `[]`        |
| `cas.ingressAPI.tls`            | TLS configuration ref: https://kubernetes.io/docs/concepts/services-networking/ingress/#tls | `[]`        |

### CAS Misc

| Name                                                | Description                   | Value   |
| --------------------------------------------------- | ----------------------------- | ------- |
| `cas.resources.limits`                              | Container resource limits     | `{}`    |
| `cas.resources.requests`                            | Container resource requests   | `{}`    |
| `cas.autoscaling.enabled`                           | Enable deployment autoscaling | `false` |
| `cas.autoscaling.minReplicas`                       | Minimum number of replicas    | `1`     |
| `cas.autoscaling.maxReplicas`                       | Maximum number of replicas    | `100`   |
| `cas.autoscaling.targetCPUUtilizationPercentage`    | Target CPU percentage         | `80`    |
| `cas.autoscaling.targetMemoryUtilizationPercentage` | Target CPU memory             | `80`    |

### Dependencies 

| Name                                 | Description                                                                                            | Value          |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------ | -------------- |
| `postgresql.enabled`                 | Switch to enable or disable the PostgreSQL helm chart                                                  | `true`         |
| `postgresql.auth.enablePostgresUser` | Assign a password to the "postgres" admin user. Otherwise, remote access will be blocked for this user | `false`        |
| `postgresql.auth.username`           | Name for a custom user to create                                                                       | `chainloop`    |
| `postgresql.auth.password`           | Password for the custom user to create                                                                 | `chainlooppwd` |
| `postgresql.auth.database`           | Name for a custom database to create                                                                   | `chainloop-cp` |
| `postgresql.auth.existingSecret`     | Name of existing secret to use for PostgreSQL credentials                                              | `""`           |
| `vault.server.dev.enabled`           | Enable development mode (unsealed, in-memory, insecure)                                                | `true`         |
| `vault.server.dev.devRootToken`      | Connection token                                                                                       | `notapassword` |


## License

Copyright &copy; 2023 The Chainloop Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

<http://www.apache.org/licenses/LICENSE-2.0>

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.