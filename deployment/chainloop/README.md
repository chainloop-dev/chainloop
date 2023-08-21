# Chainloop Helm Chart

[Chainloop](https://github.com/chainloop-dev/chainloop) is an open-source software supply chain control plane, a single source of truth for artifacts plus a declarative attestation crafting process.

## Introduction

This chart bootstraps a [Chainloop](https://github.com/chainloop-dev/chainloop) deployment on a [Kubernetes](https://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- PV provisioner support in the underlying infrastructure (If built-in PostgreSQL is enabled)

Compatibility with the following Ingress Controllers has been verified, other controllers might or might not work.

- [Nginx Ingress Controller](https://kubernetes.github.io/ingress-nginx/)
- [Traefik](https://doc.traefik.io/traefik/providers/kubernetes-ingress/)

## TL;DR

Deploy Chainloop in [development mode](#development) by running

```console
helm install [RELEASE_NAME] oci://ghcr.io/chainloop-dev/charts/chainloop \
    --set development=true \
    --set controlplane.auth.oidc.url=[OIDC URL] \
    --set controlplane.auth.oidc.clientID=[clientID] \
    --set controlplane.auth.oidc.clientSecret=[clientSecret]
```

> **CAUTION**: Do not use this mode in production, for that, use the [standard mode](#standard-default) instead.

## Installing the Chart

This chart comes in **two flavors**, `standard` and [`development`](#development).

### Standard (default)

![Deployment](../../docs/img/deployment.png)

The default deployment mode relies on external dependencies to be available in advance.

The Helm Chart in this mode includes

- Chainloop [Controlplane](https://github.com/chainloop-dev/chainloop/tree/main/app/controlplane)
- Chainloop [Artifact proxy](https://github.com/chainloop-dev/chainloop/tree/main/app/artifact-cas)
- A PostgreSQL dependency enabled by default

During installation, you'll need to provide

- Open ID Connect Identity Provider (IDp) settings i.e [Auth0 settings](https://auth0.com/docs/get-started/applications/application-settings#basic-information)
- Connection settings for a secrets storage backend, either [Hashicorp Vault](https://www.vaultproject.io/) or [AWS Secret Manager](https://aws.amazon.com/secrets-manager)
- ECDSA (ES512) key-pair used for Controlplane <-> CAS Authentication

Instructions on how to create the ECDSA keypair can be found [here](#generate-a-ecdsa-key-pair).

#### Installation Examples

> **NOTE**: **We do not recommend passing nor storing sensitive data in plain text**. For production, please consider having your overrides encrypted with tools such as [Sops](https://github.com/mozilla/sops), [Helm Secrets](https://github.com/jkroepke/helm-secrets) or [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets).

Deploy Chainloop configured to talk to the bundled PostgreSQL an external OIDC IDp and a Vault instance.

```console
helm install [RELEASE_NAME] oci://ghcr.io/chainloop-dev/charts/chainloop \
    # Open ID Connect (OIDC)
    --set controlplane.auth.oidc.url=[OIDC URL] \
    --set controlplane.auth.oidc.clientID=[clientID] \
    --set controlplane.auth.oidc.clientSecret=[clientSecret] \
    # Secrets backend
    --set secretsBackend.vault.address="https://[vault address]:8200" \
    --set secretsBackend.vault.token=[token] \
    # Server Auth KeyPair
    --set casJWTPrivateKey="$(cat private.ec.key)" \
    --set casJWTPublicKey="$(cat public.pem)"
```

Deploy using AWS secret manager instead of Vault

```console
helm install [RELEASE_NAME] oci://ghcr.io/chainloop-dev/charts/chainloop \
    # Open ID Connect (OIDC)
    # ...
    # Secrets backend
    --set secretsBackend.backend=awsSecretManager \
    --set secretsBackend.awsSecretManager.accessKey=[AWS ACCESS KEY ID] \
    --set secretsBackend.awsSecretManager.secretKey=[AWS SECRET KEY] \
    --set secretsBackend.awsSecretManager.region=[AWS region]\
    # Server Auth KeyPair
    # ...
```

Deploy using GCP secret manager instead of Vault

```console
helm install [RELEASE_NAME] oci://ghcr.io/chainloop-dev/charts/chainloop \
    # Open ID Connect (OIDC)
    # ...
    # Secrets backend
    --set secretsBackend.backend=gcpSecretManager \
    --set secretsBackend.gcpSecretManager.projectId=[GCP Project ID] \
    --set secretsBackend.gcpSecretManager.serviceAccountKey=[GCP Auth KEY] \
    # Server Auth KeyPair
    # ...
```

Connect to an external PostgreSQL database instead

```console
helm install [RELEASE_NAME] oci://ghcr.io/chainloop-dev/charts/chainloop \
    # Open ID Connect (OIDC)
    # ...
    # Secrets backend
    # ...
    # Server Auth KeyPair
    # ...
    # External DB setup
    --set postgresql.enabled=false \
    --set controlplane.externalDatabase.host=[DB_HOST] \
    --set controlplane.externalDatabase.user=[DB_USER] \
    --set controlplane.externalDatabase.password=[DB_PASSWORD] \
    --set controlplane.externalDatabase.database=[DB_NAME]
```

### Development

To provide an easy way to give Chainloop a try, this Helm Chart has an **opt-in development** mode that can be enabled with the flag `development=true`

> IMPORTANT: DO NOT USE THIS MODE IN PRODUCTION

![Deployment](../../docs/img/deployment-dev.png)

The Helm Chart in this mode includes

- Chainloop [Controlplane](https://github.com/chainloop-dev/chainloop/tree/main/app/controlplane)
- Chainloop [Artifact proxy](https://github.com/chainloop-dev/chainloop/tree/main/app/artifact-cas)
- A PostgreSQL dependency enabled by default
- **A pre-configured Hashicorp Vault instance running in development mode (unsealed, in-memory, insecure)**

> **CAUTION**: Do not use this mode in production, for that, use the [standard mode](#standard-default) instead.

During installation, you'll need to provide

- Open ID Connect Identity Provider (IDp) settings i.e [Auth0 settings](https://auth0.com/docs/get-started/applications/application-settings#basic-information)
- ~~Connection settings for a secrets storage backend, either [Hashicorp Vault](https://www.vaultproject.io/) or [AWS Secret Manager](https://aws.amazon.com/secrets-manager)~~
- ~~ECDSA (ES512) key-pair used for Controlplane <-> CAS Authentication~~

#### Installation Examples

Deploy by leveraging built-in Vault and PostgreSQL instances

```console
helm install [RELEASE_NAME] oci://ghcr.io/chainloop-dev/charts/chainloop \
    --set development=true \
    --set controlplane.auth.oidc.url=[OIDC URL] \
    --set controlplane.auth.oidc.clientID=[clientID] \
    --set controlplane.auth.oidc.clientSecret=[clientSecret]
```
## How to guides

### Generate a ECDSA key-pair

An ECDSA key-pair is required to perform authentication between the control-plane and the Artifact CAS

You can generate both the private and public keys by running

```bash
# Private Key (private.ec.key)
openssl ecparam -name secp521r1 -genkey -noout -out private.ec.key
# Public Key (public.pem)
openssl ec -in private.ec.key -pubout -out public.pem
```

Then, you can either provide it in a custom `values.yaml` file override

```yaml
casJWTPrivateKey: |-
    -----BEGIN EC PRIVATE KEY-----
    REDACTED
    -----END EC PRIVATE KEY-----
casJWTPublicKey: |
    -----BEGIN PUBLIC KEY-----
    REDACTED
    -----END PUBLIC KEY-----
```

or as shown before, provide them as imperative inputs during Helm Install/Upgrade `--set casJWTPrivateKey="$(cat private.ec.key)"--set casJWTPublicKey="$(cat public.pem)"`

### Enable a custom domain with TLS

Chainloop uses three endpoints so we'll need to enable the ingress resource for each one of them.

See below an example of a `values.yaml` override

```yaml
controlplane:
  ingress:
    enabled: true
    hostname: cp.chainloop.dev

  ingressAPI:
    enabled: true
    hostname: api.cp.chainloop.dev

cas:
    ingressAPI:
    enabled: true
    hostname: api.cas.chainloop.dev
```

A complete setup that uses

- NGINX as ingress Controller https://kubernetes.github.io/ingress-nginx/
- [cert-manager](https://cert-manager.io/) as TLS provider

would look like

```yaml
controlplane:
  ingress:
    enabled: true
    tls: true
    ingressClassName: nginx
    hostname: cp.chainloop.dev
    annotations:
      # This depends on your configured issuer 
      cert-manager.io/cluster-issuer: "letsencrypt-prod"

  ingressAPI:
    enabled: true
    tls: true
    ingressClassName: nginx
    hostname: api.cp.chainloop.dev
    annotations:
      nginx.ingress.kubernetes.io/backend-protocol: "GRPC"
      cert-manager.io/cluster-issuer: "letsencrypt-prod"
          
cas:
  ingressAPI:
    enabled: true
    tls: true
    ingressClassName: nginx
    hostname: api.cas.chainloop.dev
    annotations:
      nginx.ingress.kubernetes.io/backend-protocol: "GRPC"
      cert-manager.io/cluster-issuer: "letsencrypt-prod"
      # limit the size of the files that go through the proxy
      # 0 means to not check the size of the request so we do not get 413 error.
      # For now we are going to set a limit on 100MB files
      # Even though we send data in chunks of 1MB, this size refers to all the data sent in the streaming connection
      nginx.ingress.kubernetes.io/proxy-body-size: "100m"
```

Remember, once you have set up your domain, make sure you use the [CLI pointing](#configure-chainloop-cli-to-point-to-your-instance) to it instead of the defaults.

### Connect to an external PostgreSQL database

```yaml
# Disable built-in DB
postgresql:
  enabled: false

# Provide with external connection
controlplane:
    externalDatabase:
        host: 1.2.3.4
        port: 5432
        user: chainloop
        password: [REDACTED]
        database: chainloop-controlplane-prod
```

Alternatively, if you are using [Google Cloud SQL](https://cloud.google.com/sql) and you are running Chainloop in Google Kubernetes Engine. You can connect instead via [a proxy](https://cloud.google.com/sql/docs/mysql/connect-kubernetes-engine#proxy)

This method can also be easily enabled in this chart by doing

```yaml
# Disable built-in DB
postgresql:
  enabled: false

# Provide with external connection
controlplane:
    sqlProxy:
        # Inject the proxy sidecar
        enabled: true
        ## @param controlplane.sqlProxy.connectionName Google Cloud SQL connection name
        connectionName: "my-sql-instance"
    # Then you'll need to configure your DB settings to use the proxy IP address
    externalDatabase:
        host: [proxy-sidecar-ip-address]
        port: 5432
        user: chainloop
        password: [REDACTED]
        database: chainloop-controlplane-prod
```

### Use AWS secret manager

You can swap the secret manager backend with the following settings

```yaml
secretsBackend:
    backend: awsSecretManager
    awsSecretManager:
        accessKey: [KEY]
        secretKey: [SECRET]
        region: [REGION]
```

### Use GCP secret manager

You can swap the secret manager backend with the following settings

```yaml
secretsBackend:
    backend: gcpSecretManager
    gcpSecretManager:
        projectId: [PROJECT_ID]
        serviceAccountKey: [KEY]
```

### Send exceptions to Sentry

```yaml
sentry:
    enabled: true
    dsn: [your secret sentry project DSN URL]
    environment: production
```

### Enable Prometheus Monitoring in GKE

Chainloop exposes Prometheus compatible `/metrics` endpoints that can be easily scraped by a Prometheus data collector Server.

Google Cloud has a [managed Prometheus offering](https://cloud.google.com/stackdriver/docs/managed-prometheus/setup-managed) that could be easily enabled by setting `--set GKEMonitoring.enabled=true`. This will inject the required `PodMonitoring` custom resources.

### Configure Chainloop CLI to point to your instance

Once you have your instance of Chainloop deployed, you need to configure the [CLI](https://github.com/chainloop-dev/chainloop/releases) to point to both the CAS and the Control plane gRPC APIs like this.

```
chainloop config save \
  --control-plane my-controlplane.acme.com:443 \
  --artifact-cas cas.acme.com:443
```

## Parameters

### Common parameters

| Name                    | Description                                                                                                                                                            | Value        |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| `kubeVersion`           | Override Kubernetes version                                                                                                                                            | `""`         |
| `development`           | Deploys Chainloop pre-configured FOR DEVELOPMENT ONLY. It includes a Vault instance in development mode and pre-configured authentication certificates and passphrases | `false`      |
| `GKEMonitoring.enabled` | Enable GKE podMonitoring (prometheus.io scrape) to scrape the controlplane and CAS /metrics endpoints                                                                  | `false`      |
| `sentry.enabled`        | Enable sentry.io alerting                                                                                                                                              | `false`      |
| `sentry.dsn`            | DSN endpoint https://docs.sentry.io/product/sentry-basics/dsn-explainer/                                                                                               | `""`         |
| `sentry.environment`    | Environment tag                                                                                                                                                        | `production` |

### Secrets Backend

| Name                                                | Description                                                               | Value       |
| --------------------------------------------------- | ------------------------------------------------------------------------- | ----------- |
| `secretsBackend.backend`                            | Secrets backend type ("vault", "awsSecretManager" or "gcpSecretManager")  | `vault`     |
| `secretsBackend.secretPrefix`                       | Prefix that will be pre-pended to all secrets in the storage backend      | `chainloop` |
| `secretsBackend.vault.address`                      | Vault address                                                             |             |
| `secretsBackend.vault.token`                        | Vault authentication token                                                |             |
| `secretsBackend.awsSecretManager.accessKey`         | AWS Access KEY ID                                                         |             |
| `secretsBackend.awsSecretManager.secretKey`         | AWS Secret Key                                                            |             |
| `secretsBackend.awsSecretManager.region`            | AWS Secret Manager Region                                                 |             |
| `secretsBackend.gcpSecretManager.projectId`         | GCP Project ID                                                            |             |
| `secretsBackend.gcpSecretManager.serviceAccountKey` | GCP Auth Key                                                              |             |

### Authentication

| Name               | Description                                                            | Value |
| ------------------ | ---------------------------------------------------------------------- | ----- |
| `casJWTPrivateKey` | ECDSA (ES512) private key used for Controlplane <-> CAS Authentication | `""`  |
| `casJWTPublicKey`  | ECDSA (ES512) public key                                               | `""`  |

### Control Plane

| Name                            | Description                                                                         | Value                                           |
| ------------------------------- | ----------------------------------------------------------------------------------- | ----------------------------------------------- |
| `controlplane.replicaCount`     | Number of replicas                                                                  | `2`                                             |
| `controlplane.image.repository` | FQDN uri for the image                                                              | `ghcr.io/chainloop-dev/chainloop/control-plane` |
| `controlplane.image.tag`        | Image tag (immutable tags are recommended). If no set chart.appVersion will be used |                                                 |
| `controlplane.pluginsDir`       | Directory where to look for plugins                                                 | `/plugins`                                      |

### Control Plane Database

| Name                                     | Description                                                                                           | Value   |
| ---------------------------------------- | ----------------------------------------------------------------------------------------------------- | ------- |
| `controlplane.externalDatabase`          | External PostgreSQL configuration. These values are only used when postgresql.enabled is set to false |         |
| `controlplane.externalDatabase.host`     | Database host                                                                                         | `""`    |
| `controlplane.externalDatabase.port`     | Database port number                                                                                  | `5432`  |
| `controlplane.externalDatabase.user`     | Non-root username                                                                                     | `""`    |
| `controlplane.externalDatabase.database` | Database name                                                                                         | `""`    |
| `controlplane.externalDatabase.password` | Password for the non-root username                                                                    | `""`    |
| `controlplane.sqlProxy.enabled`          | Enable sidecar to connect to DB via Google Cloud SQL proxy                                            | `false` |
| `controlplane.sqlProxy.connectionName`   | Google Cloud SQL connection name                                                                      | `""`    |
| `controlplane.sqlProxy.resources`        | Sidecar container resources                                                                           | `{}`    |

### Control Plane Authentication

| Name                                  | Description                                                                                            | Value |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------ | ----- |
| `controlplane.auth.passphrase`        | Passphrase used to sign the Auth Tokens generated by the controlplane. Leave empty for auto-generation | `""`  |
| `controlplane.auth.oidc.url`          | Full authentication path, it should match the issuer URL of the Identity provider (IDp)                | `""`  |
| `controlplane.auth.oidc.clientID`     | OIDC IDp clientID                                                                                      | `""`  |
| `controlplane.auth.oidc.clientSecret` | OIDC IDp clientSecret                                                                                  | `""`  |

### Control Plane Networking

| Name                                       | Description                                                                                                                      | Value                    |
| ------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------ |
| `controlplane.service.type`                | Service type                                                                                                                     | `ClusterIP`              |
| `controlplane.service.port`                | Service port                                                                                                                     | `80`                     |
| `controlplane.service.targetPort`          | Service target Port                                                                                                              | `http`                   |
| `controlplane.service.nodePorts.http`      | Node port for HTTP. NOTE: choose port between <30000-32767>                                                                      |                          |
| `controlplane.serviceAPI.type`             | Service type                                                                                                                     | `ClusterIP`              |
| `controlplane.serviceAPI.port`             | Service port                                                                                                                     | `80`                     |
| `controlplane.serviceAPI.targetPort`       | Service target Port                                                                                                              | `grpc`                   |
| `controlplane.serviceAPI.annotations`      | Service annotations                                                                                                              |                          |
| `controlplane.serviceAPI.nodePorts.http`   | Node port for HTTP. NOTE: choose port between <30000-32767>                                                                      |                          |
| `controlplane.ingress.enabled`             | Enable ingress record generation for %%MAIN_CONTAINER_NAME%%                                                                     | `false`                  |
| `controlplane.ingress.pathType`            | Ingress path type                                                                                                                | `ImplementationSpecific` |
| `controlplane.ingress.hostname`            | Default host for the ingress record                                                                                              | `cp.dev.local`           |
| `controlplane.ingress.ingressClassName`    | IngressClass that will be be used to implement the Ingress (Kubernetes 1.18+)                                                    | `""`                     |
| `controlplane.ingress.path`                | Default path for the ingress record                                                                                              | `/`                      |
| `controlplane.ingress.annotations`         | Additional annotations for the Ingress resource. To enable certificate autogeneration, place here your cert-manager annotations. | `{}`                     |
| `controlplane.ingress.tls`                 | Enable TLS configuration for the host defined at `controlplane.ingress.hostname` parameter                                       | `false`                  |
| `controlplane.ingress.selfSigned`          | Create a TLS secret for this ingress record using self-signed certificates generated by Helm                                     | `false`                  |
| `controlplane.ingress.extraHosts`          | An array with additional hostname(s) to be covered with the ingress record                                                       | `[]`                     |
| `controlplane.ingress.extraPaths`          | An array with additional arbitrary paths that may need to be added to the ingress under the main host                            | `[]`                     |
| `controlplane.ingress.extraTls`            | TLS configuration for additional hostname(s) to be covered with this ingress record                                              | `[]`                     |
| `controlplane.ingress.secrets`             | Custom TLS certificates as secrets                                                                                               | `[]`                     |
| `controlplane.ingress.extraRules`          | Additional rules to be covered with this ingress record                                                                          | `[]`                     |
| `controlplane.ingressAPI.enabled`          | Enable ingress record generation for %%MAIN_CONTAINER_NAME%%                                                                     | `false`                  |
| `controlplane.ingressAPI.pathType`         | Ingress path type                                                                                                                | `ImplementationSpecific` |
| `controlplane.ingressAPI.hostname`         | Default host for the ingress record                                                                                              | `api.cp.dev.local`       |
| `controlplane.ingressAPI.ingressClassName` | IngressClass that will be be used to implement the Ingress (Kubernetes 1.18+)                                                    | `""`                     |
| `controlplane.ingressAPI.path`             | Default path for the ingress record                                                                                              | `/`                      |
| `controlplane.ingressAPI.annotations`      | Additional annotations for the Ingress resource. To enable certificate autogeneration, place here your cert-manager annotations. |                          |
| `controlplane.ingressAPI.tls`              | Enable TLS configuration for the host defined at `controlplane.ingress.hostname` parameter                                       | `false`                  |
| `controlplane.ingressAPI.selfSigned`       | Create a TLS secret for this ingress record using self-signed certificates generated by Helm                                     | `false`                  |
| `controlplane.ingressAPI.extraHosts`       | An array with additional hostname(s) to be covered with the ingress record                                                       | `[]`                     |
| `controlplane.ingressAPI.extraPaths`       | An array with additional arbitrary paths that may need to be added to the ingress under the main host                            | `[]`                     |
| `controlplane.ingressAPI.extraTls`         | TLS configuration for additional hostname(s) to be covered with this ingress record                                              | `[]`                     |
| `controlplane.ingressAPI.secrets`          | Custom TLS certificates as secrets                                                                                               | `[]`                     |
| `controlplane.ingressAPI.extraRules`       | Additional rules to be covered with this ingress record                                                                          | `[]`                     |

### Controlplane Misc

| Name                                                         | Description                        | Value   |
| ------------------------------------------------------------ | ---------------------------------- | ------- |
| `controlplane.resources.limits.cpu`                          | Container resource limits CPU      | `250m`  |
| `controlplane.resources.limits.memory`                       | Container resource limits memory   | `512Mi` |
| `controlplane.resources.requests.cpu`                        | Container resource requests CPU    | `250m`  |
| `controlplane.resources.requests.memory`                     | Container resource requests memory | `512Mi` |
| `controlplane.autoscaling.enabled`                           | Enable deployment autoscaling      | `false` |
| `controlplane.autoscaling.minReplicas`                       | Minimum number of replicas         | `1`     |
| `controlplane.autoscaling.maxReplicas`                       | Maximum number of replicas         | `100`   |
| `controlplane.autoscaling.targetCPUUtilizationPercentage`    | Target CPU percentage              | `80`    |
| `controlplane.autoscaling.targetMemoryUtilizationPercentage` | Target CPU memory                  | `80`    |

### Artifact Content Addressable (CAS) API

| Name                   | Description                                                                         | Value                                          |
| ---------------------- | ----------------------------------------------------------------------------------- | ---------------------------------------------- |
| `cas.replicaCount`     | Number of replicas                                                                  | `2`                                            |
| `cas.image.repository` | FQDN uri for the image                                                              | `ghcr.io/chainloop-dev/chainloop/artifact-cas` |
| `cas.image.tag`        | Image tag (immutable tags are recommended). If no set chart.appVersion will be used |                                                |

### CAS Networking

| Name                              | Description                                                                                                                      | Value                    |
| --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------ |
| `cas.service.type`                | Service type                                                                                                                     | `ClusterIP`              |
| `cas.service.port`                | Service port                                                                                                                     | `80`                     |
| `cas.service.targetPort`          | Service target Port                                                                                                              | `http`                   |
| `cas.service.nodePorts.http`      | Node port for HTTP. NOTE: choose port between <30000-32767>                                                                      |                          |
| `cas.serviceAPI.type`             | Service type                                                                                                                     | `ClusterIP`              |
| `cas.serviceAPI.port`             | Service port                                                                                                                     | `80`                     |
| `cas.serviceAPI.targetPort`       | Service target Port                                                                                                              | `grpc`                   |
| `cas.serviceAPI.annotations`      | Service annotations                                                                                                              |                          |
| `cas.serviceAPI.nodePorts.http`   | Node port for HTTP. NOTE: choose port between <30000-32767>                                                                      |                          |
| `cas.ingress.enabled`             | Enable ingress record generation for %%MAIN_CONTAINER_NAME%%                                                                     | `false`                  |
| `cas.ingress.pathType`            | Ingress path type                                                                                                                | `ImplementationSpecific` |
| `cas.ingress.hostname`            | Default host for the ingress record                                                                                              | `cas.dev.local`          |
| `cas.ingress.ingressClassName`    | IngressClass that will be be used to implement the Ingress (Kubernetes 1.18+)                                                    | `""`                     |
| `cas.ingress.path`                | Default path for the ingress record                                                                                              | `/`                      |
| `cas.ingress.annotations`         | Additional annotations for the Ingress resource. To enable certificate autogeneration, place here your cert-manager annotations. | `{}`                     |
| `cas.ingress.tls`                 | Enable TLS configuration for the host defined at `controlplane.ingress.hostname` parameter                                       | `false`                  |
| `cas.ingress.selfSigned`          | Create a TLS secret for this ingress record using self-signed certificates generated by Helm                                     | `false`                  |
| `cas.ingress.extraHosts`          | An array with additional hostname(s) to be covered with the ingress record                                                       | `[]`                     |
| `cas.ingress.extraPaths`          | An array with additional arbitrary paths that may need to be added to the ingress under the main host                            | `[]`                     |
| `cas.ingress.extraTls`            | TLS configuration for additional hostname(s) to be covered with this ingress record                                              | `[]`                     |
| `cas.ingress.secrets`             | Custom TLS certificates as secrets                                                                                               | `[]`                     |
| `cas.ingress.extraRules`          | Additional rules to be covered with this ingress record                                                                          | `[]`                     |
| `cas.ingressAPI.enabled`          | Enable ingress record generation for %%MAIN_CONTAINER_NAME%%                                                                     | `false`                  |
| `cas.ingressAPI.pathType`         | Ingress path type                                                                                                                | `ImplementationSpecific` |
| `cas.ingressAPI.hostname`         | Default host for the ingress record                                                                                              | `api.cas.dev.local`      |
| `cas.ingressAPI.ingressClassName` | IngressClass that will be be used to implement the Ingress (Kubernetes 1.18+)                                                    | `""`                     |
| `cas.ingressAPI.path`             | Default path for the ingress record                                                                                              | `/`                      |
| `cas.ingressAPI.annotations`      | Additional annotations for the Ingress resource. To enable certificate autogeneration, place here your cert-manager annotations. |                          |
| `cas.ingressAPI.tls`              | Enable TLS configuration for the host defined at `controlplane.ingress.hostname` parameter                                       | `false`                  |
| `cas.ingressAPI.selfSigned`       | Create a TLS secret for this ingress record using self-signed certificates generated by Helm                                     | `false`                  |
| `cas.ingressAPI.extraHosts`       | An array with additional hostname(s) to be covered with the ingress record                                                       | `[]`                     |
| `cas.ingressAPI.extraPaths`       | An array with additional arbitrary paths that may need to be added to the ingress under the main host                            | `[]`                     |
| `cas.ingressAPI.extraTls`         | TLS configuration for additional hostname(s) to be covered with this ingress record                                              | `[]`                     |
| `cas.ingressAPI.secrets`          | Custom TLS certificates as secrets                                                                                               | `[]`                     |
| `cas.ingressAPI.extraRules`       | Additional rules to be covered with this ingress record                                                                          | `[]`                     |

### CAS Misc

| Name                                                | Description                        | Value   |
| --------------------------------------------------- | ---------------------------------- | ------- |
| `cas.resources.limits.cpu`                          | Container resource limits CPU      | `250m`  |
| `cas.resources.limits.memory`                       | Container resource limits memory   | `512Mi` |
| `cas.resources.requests.cpu`                        | Container resource requests CPU    | `250m`  |
| `cas.resources.requests.memory`                     | Container resource requests memory | `512Mi` |
| `cas.autoscaling.enabled`                           | Enable deployment autoscaling      | `false` |
| `cas.autoscaling.minReplicas`                       | Minimum number of replicas         | `1`     |
| `cas.autoscaling.maxReplicas`                       | Maximum number of replicas         | `100`   |
| `cas.autoscaling.targetCPUUtilizationPercentage`    | Target CPU percentage              | `80`    |
| `cas.autoscaling.targetMemoryUtilizationPercentage` | Target CPU memory                  | `80`    |

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
