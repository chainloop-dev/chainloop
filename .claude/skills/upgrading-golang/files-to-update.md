# Files to Update During Go Version Upgrade

This reference lists all files that must be updated when upgrading Go versions.

## Source Code

### Go Modules
- `./go.mod` - Main project go.mod

**DO NOT UPDATE**:
- `./extras/dagger/go.mod` - Dagger module (per project policy)

## Docker Images

### Dockerfiles (Golang)
- `./app/artifact-cas/Dockerfile.goreleaser`
- `./app/controlplane/Dockerfile.goreleaser`
- `./app/cli/Dockerfile.goreleaser`

Update pattern in all:
```dockerfile
FROM golang:X.XX.X@sha256:DIGEST AS builder
```

## golangci-lint (Go minor upgrades)

- `./.github/workflows/lint.yml` - every `version:` of `golangci/golangci-lint-action` (main module, apps, and dagger module jobs)
- `./common.mk` - golangci-lint install version in the `init` target

Keep both on the same golangci-lint version.

## GitHub Actions

No change needed for Go itself: `test.yml`, `lint.yml`, `release.yaml` and `codeql.yml` use `go-version-file: 'go.mod'`.

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

## Documentation

### Project Documentation
- `./CLAUDE.md` - Update "Key Technologies" section:
  ```markdown
  - **Language**: Go X.XX.X
  ```

## Summary

**Files to update for Go**: 5 files
- 1 go.mod file
- 3 Dockerfiles (Golang)
- 1 documentation file

**golangci-lint (Go minor upgrades)**: 2 files
- 1 GitHub Actions workflow (lint.yml)
- 1 Makefile (common.mk)

**Optional Atlas upgrade**: 2 files
- 1 Dockerfile (Atlas migrations)
- 1 Makefile (Atlas CLI in make init)
