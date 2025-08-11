# Go Version Upgrade Process

This document outlines the comprehensive process for upgrading Go versions across the Chainloop project.

NOTE

- do not upgrade dagger module

## Overview

The Chainloop project uses Go in multiple components requiring updates across:
- Source code (go.mod files)
- Docker images  
- CI/CD pipelines (GitHub Actions)
- Documentation

## Step-by-Step Process

### 1. Get Latest Go Version and Docker Image Digest

```bash
# Check latest Go version at https://go.dev/doc/devel/release
# Get Docker image SHA256 digest
docker pull golang:1.24.6
# Note the SHA256 from output: sha256:2c89c41fb9efc3807029b59af69645867cfe978d2b877d475be0d72f6c6ce6f6
```

### 2. Update Source Code

Update all `go.mod` files in the project:
- `./go.mod` - Main project 
- `./extras/dagger/go.mod` - Dagger module

Change the `go` directive to new version:
```go
go 1.24.6
```

### 3. Update Docker Images  

Update all Dockerfiles with new version and SHA256:
- `./app/artifact-cas/Dockerfile`
- `./app/artifact-cas/Dockerfile.goreleaser`
- `./app/controlplane/Dockerfile`
- `./app/controlplane/Dockerfile.goreleaser`
- `./app/cli/Dockerfile.goreleaser`

Replace `FROM` lines with:
```dockerfile
FROM golang:1.24.6@sha256:2c89c41fb9efc3807029b59af69645867cfe978d2b877d475be0d72f6c6ce6f6 AS builder
```

### 4. Update GitHub Actions

Update go-version in all workflow files:
- `./.github/workflows/lint.yml`
- `./.github/workflows/test.yml`
- `./.github/workflows/release.yaml`
- `./.github/workflows/codeql.yml`
- `./docs/examples/ci-workflows/github.yaml`

Find and replace:
```yaml
go-version: "1.24.4"  # old version
```
with:
```yaml  
go-version: "1.24.6"  # new version
```

### 5. Update Documentation

Update version reference in `./CLAUDE.md` - "Key Technologies" section:
```markdown
- **Language**: Go 1.24.6
```

### 6. Test and Verify

```bash
make test    # Ensure compatibility 
make lint    # Check code quality
```

## Important Notes

1. **SHA256 Verification**: Always use SHA256 digests for Docker images for security and reproducibility
2. **Test Thoroughly**: Go upgrades can introduce breaking changes  
3. **Multiple Components**: CLI, Control Plane, and Artifact CAS all use Go
4. **Dagger Module**: Has separate go.mod that needs updating
5. **Development Environment**: Compose files use pre-built images, don't need Go updates

## Files Updated in This Process

### Source Code
- `./go.mod`

### Docker Images
- `./app/artifact-cas/Dockerfile`
- `./app/artifact-cas/Dockerfile.goreleaser`
- `./app/controlplane/Dockerfile`
- `./app/controlplane/Dockerfile.goreleaser`
- `./app/cli/Dockerfile.goreleaser`

### CI/CD Workflows  
- `./.github/workflows/lint.yml`
- `./.github/workflows/test.yml`
- `./.github/workflows/release.yaml`
- `./.github/workflows/codeql.yml`
- `./docs/examples/ci-workflows/github.yaml`

### Documentation
- `./CLAUDE.md`