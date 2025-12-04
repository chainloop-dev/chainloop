---
name: upgrading-golang
description: Upgrades Go version across the entire Chainloop codebase including source files, Docker images, CI/CD workflows, and documentation. Use when the user mentions upgrading Go, golang version, or updating Go compiler version.
---

# Upgrading Golang Version

This skill automates the comprehensive Go version upgrade process across all components of the Chainloop project.

## Process

### 1. Confirm Target Versions

Ask the user:
1. What Go version they want to upgrade to (e.g., "1.25.3")
2. Whether they also want to upgrade Atlas migrations Docker image (if yes, ask for target Atlas version, e.g., "0.38.0")

### 2. Get Docker Image Digest

Pull the official golang Docker image and extract its SHA256 digest:

```bash
docker pull golang:X.XX.X
```

Extract the SHA256 digest from the output (format: `sha256:abc123...`).

### 3. Update Source Code

Update the `go` directive in:
- `./go.mod`

**IMPORTANT**: Do NOT update `./extras/dagger/go.mod` per project policy.

Pattern to replace:
```go
go X.XX.X
```

### 4. Update Docker Images

Update all Dockerfiles with the new version and SHA256 digest. See [files-to-update.md](files-to-update.md) for the complete list.

Pattern to replace:
```dockerfile
FROM golang:X.XX.X@sha256:OLD_DIGEST AS builder
```

With:
```dockerfile
FROM golang:X.XX.X@sha256:NEW_DIGEST AS builder
```

### 5. Update GitHub Actions

Update `go-version` in all workflow YAML files. See [files-to-update.md](files-to-update.md) for the complete list.

Pattern to replace:
```yaml
go-version: "X.XX.X"
```

### 6. Update Documentation

Update the version reference in `./CLAUDE.md` under "Key Technologies":
```markdown
- **Language**: Go X.XX.X. To know how to upgrade go version, see docs/runbooks
```

### 7. Update Atlas Docker Image and CLI (Optional)

If the user requested an Atlas upgrade:

**7a. Pull the Atlas Docker image and extract its SHA256 digest:**

```bash
docker pull arigaio/atlas:X.XX.X
```

Extract the SHA256 digest from the output (format: `sha256:abc123...`).

**7b. Update `./app/controlplane/Dockerfile.migrations`:**

Pattern to replace:
```dockerfile
# from: arigaio/atlas:X.XX.X
# docker run arigaio/atlas@sha256:OLD_DIGEST version
# atlas version vX.XX.X
FROM arigaio/atlas@sha256:OLD_DIGEST as base
```

With:
```dockerfile
# from: arigaio/atlas:X.XX.X
# docker run arigaio/atlas@sha256:NEW_DIGEST version
# atlas version vX.XX.X
FROM arigaio/atlas@sha256:NEW_DIGEST as base
```

**7c. Update `./common.mk` for `make init`:**

**IMPORTANT**: Before updating the version in common.mk, ALWAYS test that the Atlas version is available via the curl command:

```bash
curl -sSf https://atlasgo.sh | ATLAS_VERSION=vX.XX.X sh -s -- --version
```

If the command fails or the version is not available, do NOT update common.mk. Only the Docker image should be updated in this case.

Update the Atlas CLI installation version in the `init` target:

Pattern to replace:
```makefile
curl -sSf https://atlasgo.sh | ATLAS_VERSION=vX.XX.X sh -s -- -y
```

With the new version (note: use `v` prefix for the version):
```makefile
curl -sSf https://atlasgo.sh | ATLAS_VERSION=vX.XX.X sh -s -- -y
```

### 8. Verify Changes

Run verification commands:
```bash
make test
make lint
```

If errors occur, address them before completing the upgrade.

### 9. Final Checks

- Ensure all license headers are updated (2024 → 2024-2025 or add current year)
- Run `buf format -w` if any proto files were affected
- Run `wire ./...` if any constructor dependencies changed
- Verify `go.mod` changes with `go mod tidy`

## Important Notes

- Always use SHA256 digests for Docker images for security and reproducibility
- The dagger module (`./extras/dagger/go.mod`) must NOT be updated
- Test thoroughly as Go upgrades can introduce breaking changes
- Multiple components use Go: CLI, Control Plane, and Artifact CAS
