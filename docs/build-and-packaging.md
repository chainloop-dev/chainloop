# Chainloop Build and Packaging Overview

## Release Flow

The release process starts by pushing a **git tag** matching `v*.*.*`, which triggers the `release.yaml` GitHub Actions workflow.

```
Tag push (v1.x.x) → Tests → GoReleaser build → Signing → Helm chart bump PR
```

## Building with GoReleaser

All binaries and container images are built via [GoReleaser](https://goreleaser.com/) (`.goreleaser.yml` at the repo root).

### Binaries

| Binary | Platforms | Notes |
|--------|-----------|-------|
| `control-plane` | linux/amd64, linux/arm64 | Main backend service |
| `artifact-cas` | linux/amd64, linux/arm64 | Content-addressable storage proxy |
| `chainloop` (CLI) | darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 | Multi-platform client |
| `chainloop-plugin-discord-webhook` | linux/amd64 | Bundled into control-plane image |
| `chainloop-plugin-smtp` | linux/amd64 | Bundled into control-plane image |
| `chainloop-plugin-dependency-track` | linux/amd64 | Bundled into control-plane image |

All binaries are statically compiled (`CGO_ENABLED=0`) and stripped (`-s -w`). Version info is injected via `-ldflags` from the git tag. CLI binaries are published as GitHub Release assets alongside a `checksums.txt`.

### Container Images

GoReleaser uses **Docker buildx with QEMU** to produce multi-architecture images (amd64 + arm64), published to **GitHub Container Registry** (`ghcr.io/chainloop-dev/chainloop/`).

| Image | Base | Dockerfile |
|-------|------|------------|
| `control-plane` | `scratch` | `app/controlplane/Dockerfile.goreleaser` |
| `control-plane-migrations` | `arigaio/atlas` | `app/controlplane/Dockerfile.migrations` |
| `artifact-cas` | `scratch` | `app/artifact-cas/Dockerfile.goreleaser` |
| `cli` | `scratch` | `app/cli/Dockerfile.goreleaser` |

**Tagging strategy:**
- Platform-specific: `<image>:<tag>-amd64`, `<image>:<tag>-arm64`
- Unified manifest: `<image>:<tag>` and `<image>:latest`

### Signing

All container images and release artifacts are signed with **Cosign** using a private key.

## Building Dependency Container Images

We don't consume upstream Bitnami container images directly. Instead, we **rebuild them from source** using the Dockerfiles in the [bitnami/containers](https://github.com/bitnami/containers) repository. This gives us full control over the build process, allows us to sign images with our own key, and host them in our own registry.

This is handled by the **`build_external_container_images.yaml`** workflow, triggered manually via `workflow_dispatch`.

### How it works

1. The workflow checks out the `bitnami/containers` repo at a **pinned commit SHA** for reproducibility.
2. Builds each image for **linux/amd64 + linux/arm64** using Docker buildx.
3. Pushes to GHCR under `ghcr.io/chainloop-dev/chainloop/`.
4. Signs the resulting image with Cosign.

### Images built

| Image | Version | Bitnami Source Path |
|-------|---------|---------------------|
| `postgresql` | 16.4.0 | `bitnami/postgresql/16/debian-12` |
| `postgres-exporter` | 0.15.0 | `bitnami/postgres-exporter/0/debian-12` |
| `os-shell` | 12 | `bitnami/os-shell/12/debian-12` |
| `dex` | 2.43.1 | `bitnami/dex/2/debian-12` |
| `vault` | 1.17.3 | `bitnami/vault/1/debian-12` |
| `vault-csi-provider` | 1.4.3 | `bitnami/vault-csi-provider/1/debian-12` |
| `vault-k8s` | 1.4.2 | `bitnami/vault-k8s/1/debian-12` |
| `nginx-ingress-controller` | 1.12.1 | `bitnami/nginx-ingress-controller/1/debian-12` |
| `nginx` | 1.27.4 | `bitnami/nginx/1.27/debian-12` |

Each image is tagged with its app version, `latest`, and the git SHA.

## Helm Chart

### Location

The main Helm chart lives at **`deployment/chainloop/`**.

### Dependencies

Sub-charts are **vendored** (committed under `deployment/chainloop/charts/`) as `file://` dependencies:

| Sub-chart | Source | Condition |
|-----------|--------|-----------|
| `common` | Bitnami Common | Always (template utilities) |
| `postgresql` | Bitnami PostgreSQL | `postgresql.enabled` |
| `vault` | Bitnami Vault | `development` flag |
| `dex` | Custom | `development` flag |

All sub-chart container image references point to our own GHCR rebuilds (e.g., `ghcr.io/chainloop-dev/chainloop/postgresql:16.4.0`), not upstream Bitnami or Docker Hub.

### Chart Bumping on Release

After a successful release, the workflow creates a **PR** that:

1. Bumps `appVersion` to the new tag (e.g., `v1.79.1`).
2. Increments the chart version.
3. Updates image tag references in `values.yaml`.

This is handled by `.github/workflows/utils/bump-chart-and-dagger-version.sh`.

## Local Development Builds

```bash
# Build binaries only (snapshot, current platform)
make build_devel

# Build binaries + container images locally (no signing)
make build_devel_container_images
```
