# Files to Update During Go Version Upgrade

This reference lists all files that must be updated when upgrading Go versions.

## Source Code

### Go Modules
- `./go.mod` - Main project go.mod

**DO NOT UPDATE**:
- `./extras/dagger/go.mod` - Dagger module (per project policy)

## Docker Images

### Dockerfiles (Golang)
- `./app/artifact-cas/Dockerfile`
- `./app/artifact-cas/Dockerfile.goreleaser`
- `./app/controlplane/Dockerfile`
- `./app/controlplane/Dockerfile.goreleaser`
- `./app/cli/Dockerfile.goreleaser`

Update pattern in all:
```dockerfile
FROM golang:X.XX.X@sha256:DIGEST AS builder
```

### Atlas Files (Optional)
- `./app/controlplane/Dockerfile.migrations` - Docker image for migrations
- `./common.mk` - CLI tool installation in `make init`

Update patterns:

**Dockerfile.migrations:**
```dockerfile
# from: arigaio/atlas:X.XX.X
# docker run arigaio/atlas@sha256:DIGEST version
# atlas version vX.XX.X
FROM arigaio/atlas@sha256:DIGEST as base
```

**common.mk:**
```makefile
curl -sSf https://atlasgo.sh | ATLAS_VERSION=vX.XX.X sh -s -- -y
```

## CI/CD Workflows

### GitHub Actions
- `./.github/workflows/lint.yml`
- `./.github/workflows/test.yml`
- `./.github/workflows/release.yaml`
- `./.github/workflows/codeql.yml`
- `./docs/examples/ci-workflows/github.yaml`

Update pattern in all:
```yaml
go-version: "X.XX.X"
```

## Documentation

### Project Documentation
- `./CLAUDE.md` - Update "Key Technologies" section:
  ```markdown
  - **Language**: Go X.XX.X. To know how to upgrade go version, see docs/runbooks
  ```

## Summary

**Total files to update for Go**: 13 files
- 1 go.mod file
- 5 Dockerfiles (Golang)
- 5 GitHub Actions workflows
- 1 example workflow
- 1 documentation file

**Optional Atlas upgrade**: 2 files
- 1 Dockerfile (Atlas migrations)
- 1 Makefile (Atlas CLI in make init)
