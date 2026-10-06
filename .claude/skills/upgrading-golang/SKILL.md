---
name: upgrading-golang
description: Upgrades Go version across the entire Chainloop codebase including source files, Docker images, golangci-lint, CI/CD workflows, and documentation. Use when the user mentions upgrading Go, golang version, or updating Go compiler version, or when CI lint fails because golangci-lint was built with an older Go than the one targeted in go.mod.
---

# Upgrading Golang Version

This skill automates the comprehensive Go version upgrade process across all components of the Chainloop project.

## Process

### 1. Confirm Target Versions

Ask the user:
1. What Go version they want to upgrade to (e.g., "1.25.3"). If they give only a minor version (e.g., "1.27"), use the latest patch from `curl -s 'https://go.dev/dl/?mode=json'`.
2. Whether they also want to upgrade Atlas migrations Docker image (if yes, ask for target Atlas version, e.g., "0.38.0")

### 2. Get Docker Image Digest

Pull the official golang Docker image and extract its SHA256 digest:

```bash
docker pull golang:X.XX.X
```

Extract the SHA256 digest from the output (format: `sha256:abc123...`).

If the Docker daemon is not running, read the multi-arch index digest from the registry instead:

```bash
TOKEN=$(curl -s "https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/golang:pull" | jq -r .token)
curl -sI -H "Authorization: Bearer $TOKEN" \
  -H "Accept: application/vnd.oci.image.index.v1+json" \
  -H "Accept: application/vnd.docker.distribution.manifest.list.v2+json" \
  https://registry-1.docker.io/v2/library/golang/manifests/X.XX.X | grep -i docker-content-digest
```

### 3. Update Source Code

Update the `go` directive in:
- `./go.mod`

**IMPORTANT**: Do NOT update `./extras/dagger/go.mod` per project policy.

Pattern to replace:
```go
go X.XX.X
```

Then run `go mod tidy` and `go build ./...`.

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

### 5. Update golangci-lint

golangci-lint refuses to run when it was built with a Go version older than the `go` directive in `go.mod` ("the Go language version (goX.YY) used to build golangci-lint is lower than the targeted Go version"). On a Go minor upgrade, bump it to a release that supports the new Go version.

1. Find the first golangci-lint release that adds support for the new Go minor (its changelog has a "goX.YY support" entry): `gh release list -R golangci/golangci-lint`. Prefer the latest release.
2. Update every `version:` of `golangci/golangci-lint-action` in `.github/workflows/lint.yml`.
3. Update the install version in the `init` target of `./common.mk` to the same version.
4. Verify locally the way CI runs it (CI uses `only-new-issues`), from the repo root and from `app/cli`, `app/controlplane` and `app/artifact-cas`:

```bash
GOBIN=<scratch-dir> go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@vX.Y.Z
<scratch-dir>/golangci-lint run --new-from-rev=<upstream>/main
```

For a Go patch upgrade, the golangci-lint bump is not required.

### 6. Update Documentation

Update the version reference in `./CLAUDE.md` under "Key Technologies":
```markdown
- **Language**: Go X.XX.X
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

- Search for leftover references to the old version: `grep -rnE "golang:OLD|go OLD|OLD_MINOR\.[0-9]" --exclude=go.sum .`
- Ensure all license headers are updated (2024 → 2024-2025 or add current year)
- Run `buf format -w` if any proto files were affected
- Run `wire ./...` if any constructor dependencies changed
- Verify `go.mod` changes with `go mod tidy`

## Important Notes

- Always use SHA256 digests for Docker images for security and reproducibility
- The dagger module (`./extras/dagger/go.mod`) must NOT be updated
- GitHub Actions workflows read the Go version from `go.mod` (`go-version-file`), so they need no change for Go itself
- Test thoroughly as Go upgrades can introduce breaking changes
- Multiple components use Go: CLI, Control Plane, and Artifact CAS
