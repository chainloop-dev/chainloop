---
name: upgrading-golang
description: Upgrades Go version across the entire Chainloop codebase including source files, Docker images, CI/CD workflows, and documentation. Use when the user mentions upgrading Go, golang version, or updating Go compiler version.
---

# Upgrading Golang Version

This skill automates the comprehensive Go version upgrade process across all components of the Chainloop project.

## Process

### 1. Confirm Target Version

Ask the user for the target Go version (e.g., "1.25.0").

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

### 7. Verify Changes

Run verification commands:
```bash
make test
make lint
```

If errors occur, address them before completing the upgrade.

### 8. Final Checks

- Ensure all license headers are updated (2024 → 2024-2025 or add current year)
- Run `buf format -w` if any proto files were affected
- Run `wire ./...` if any constructor dependencies changed
- Verify `go.mod` changes with `go mod tidy`

## Important Notes

- Always use SHA256 digests for Docker images for security and reproducibility
- The dagger module (`./extras/dagger/go.mod`) must NOT be updated
- Test thoroughly as Go upgrades can introduce breaking changes
- Multiple components use Go: CLI, Control Plane, and Artifact CAS
