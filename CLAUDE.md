# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Chainloop is an open-source evidence store for Software Supply Chain attestations, SBOMs, VEX, SARIF, and other compliance artifacts. The project consists of three main components: a Control Plane (server), Artifact Content Addressable Storage (CAS), and a CLI client.

## Architecture

The codebase follows a microservices architecture with three main components and shared libraries:

### Control Plane (`app/controlplane/`)
Main backend service implementing hexagonal architecture:
- **API Layer** (`./api/`): Protobuf definitions and generated gRPC/HTTP services
- **Server Layer** (`./internal/server`): HTTP/gRPC servers, middlewares, and request handling
- **Service Layer** (`./internal/service`): Protocol buffer service implementations
- **Business Layer** (`./pkg/biz`): Core business logic and repository abstractions
  - Organizations, workflows, attestations, API tokens, groups, projects
  - CAS backend management, integrations, user access control
  - Policy evaluation and referrer management
- **Data Layer** (`./pkg/data`): Repository implementations using Ent ORM
  - PostgreSQL with Atlas migrations in `./pkg/data/ent/migrate/migrations/`
  - Ent schemas in `./pkg/data/ent/schema/`

**Dependencies**: OIDC provider, PostgreSQL, Secret storage (Vault/AWS/GCP/Azure), Artifact CAS
**Authentication**: JWT tokens, OIDC delegation, RBAC with Casbin
**Key Features**: Multi-tenancy, workflow contracts, policy as code, audit logging

### Artifact CAS (`app/artifact-cas/`)
Content Addressable Storage proxy service:
- **API Layer** (`./api/`): gRPC bytestream protocol for efficient streaming
- **Server Layer** (`./internal/server`): gRPC server setup and middlewares
- **Service Layer** (`./internal/service`): Bytestream, download, resource, and status services
- **Storage Backends** (`pkg/blobmanager/`): OCI registry, S3, Azure Blob Storage
  - Content-addressable storage with SHA256 digests
  - Multi-tenant through runtime credential selection
  - Immutable artifact storage with digest verification

**Dependencies**: Secret storage backend for OCI/blob credentials
**Authentication**: JWT tokens with upload/download permissions from Control Plane
**Key Features**: Multi-backend support, streaming uploads/downloads, content verification

### CLI (`app/cli/`)
Command-line client for both operators and CI/CD systems:
- **Commands** (`./cmd/`): Cobra-based command structure
  - `attestation_*`: Attestation crafting lifecycle (init, add, push, status, verify)
  - `workflow_*`: Workflow and contract management
  - `organization_*`: Organization and membership operations
  - `artifact_*`: Artifact upload/download operations
  - `auth_*`: Authentication and account management
- **Actions** (`./internal/action/`): Business logic implementations for each command
- **Policy Development** (`./internal/policydevel/`): Local policy development and testing
- **Telemetry** (`./internal/telemetry/`): Usage analytics with PostHog

**Key Features**: OIDC authentication, multi-environment config, plugin system, attestation crafting

### Shared Libraries (`pkg/`)
Common functionality across components:
- **Attestation** (`pkg/attestation/`): 
  - **Crafter**: Attestation creation with material collection and runner context
  - **Materials**: Support for 25+ evidence types (SBOM, SARIF, VEX, OCI images, etc.)
  - **Runners**: CI/CD platform integration (GitHub Actions, GitLab, Azure, Jenkins, etc.)
  - **Signer**: Chainloop, Cosign, SignServer integration
  - **Verifier**: Attestation verification and timestamp validation
- **Policies** (`pkg/policies/`): OPA/Rego policy evaluation engine
- **Blob Manager** (`pkg/blobmanager/`): Multi-backend storage abstraction
- **Credentials** (`pkg/credentials/`): Secret management for multiple providers
- **gRPC Connection** (`pkg/grpcconn/`): Reusable gRPC client setup

## Development Commands

### Initial Setup
```bash
make init                    # Install required development tools
```

### Building
```bash
# Root level - build all components
make all                     # Generate APIs and protobuf bindings
make api                     # Generate API proto bindings
make generate               # Generate code, APIs, configs

# Individual components
make -C app/controlplane build
make -C app/cli build
make -C app/artifact-cas build
```

### Testing
```bash
# Root level
make test                   # Run all tests

# Component-specific
make -C app/controlplane test        # All tests including integration
make -C app/controlplane test-unit   # Unit tests only (SKIP_INTEGRATION=true)
make -C app/cli test
make -C app/artifact-cas test
```

### Linting
```bash
# Root level - lint all components
make lint

# Component-specific
make -C app/controlplane lint    # golangci-lint + buf lint
make -C app/cli lint
make -C app/artifact-cas lint
```

### Development Server
```bash
# Start auxiliary services (PostgreSQL, Vault, Dex OIDC)
cd devel && docker compose up

# Run components (in separate terminals)
make -C app/controlplane run     # Runs migration_apply first
make -C app/artifact-cas run

# CLI usage
go run app/cli/main.go --insecure [command]
```

### Database Operations
```bash
cd app/controlplane
make migration_apply        # Apply migrations to local DB
make migration_sync         # Sync migrations with Ent schema
make migration_new          # Create empty migration file
make migration_lint         # Lint migration files
```

## Key Technologies

- **Language**: Go 1.25.6
- **API**: gRPC with HTTP/JSON gateway, Protocol Buffers with buf
- **Database**: PostgreSQL with Ent ORM, Atlas for migrations
- **Authentication**: OIDC, JWT tokens
- **Policy**: Open Policy Agent (OPA) with Rego
- **Storage**: Multi-backend (OCI, S3, Azure Blob, inline)
- **Cryptography**: Sigstore (Cosign, Fulcio), in-toto attestations
- **Standards**: SLSA, SBOM (SPDX/CycloneDX), SARIF, OpenVEX, CSAF

## Development Environment

### Setup Options

**Option 1: Local Development (Recommended)**
```bash
# 1. Install tools
make init

# 2. Start auxiliary services
cd devel && docker compose up

# 3. Run components locally (separate terminals)
make -C app/controlplane run
make -C app/artifact-cas run

# 4. Configure CLI
go run app/cli/main.go config save --insecure --control-plane localhost:9000 --artifact-cas localhost:9001
go run app/cli/main.go --insecure auth login
```

**Option 2: Containerized Labs Environment**
```bash
# 1. Add dex hostname to /etc/hosts
echo "127.0.0.1 dex" | sudo tee -a /etc/hosts

# 2. Run full containerized stack
docker compose -f devel/compose.labs.yml up

# 3. Extract development token from logs
docker compose -f devel/compose.labs.yml logs control-plane | grep "DEVELOPMENT USER TOKEN" -A 1

# 4. Authenticate with token
chainloop --insecure auth login --skip-browser
```

### Development Infrastructure

**Compose Configuration**:
- `compose.common.yml`: PostgreSQL 16 + Vault (in-memory dev mode)
- `compose.yml`: Adds Dex OIDC for local development
- `compose.labs.yml`: Full containerized environment with pre-built images

**Development Services**:
- **PostgreSQL**: `localhost:5432`, database: `controlplane`, user: `postgres`, no password
- **Vault**: `localhost:8200`, dev token: `notasecret` (in-memory, data lost on restart)
- **Dex OIDC**: `localhost:5556`, static users with password `"password"`
- **Control Plane**: `localhost:9000` (gRPC) / `localhost:8000` (HTTP)
- **Artifact CAS**: `localhost:9001` (gRPC) / `localhost:8001` (HTTP)
- **Optional Minio S3**: `localhost:9002` (API) / `localhost:9003` (Console) with `--profile optional`

**Development Credentials**:
- **OIDC Users**: `sarah@chainloop.local` / `john@chainloop.local` (password: `password`)
- **Signing Keys**: Development keypairs in `devel/devkeys/` (DO NOT USE IN PRODUCTION)
  - `ca.pem`/`ca.pub`: Certificate Authority
  - `cas.pem`/`cas.pub`: Artifact CAS signing
  - `freetsa.pem`: Free TSA timestamp authority
  - Self-signed certificates in `devkeys/selfsigned/`

## Testing Environment

- **Integration Tests**: Use testcontainers for isolated database testing
- **Unit Tests**: Run with `make test-unit` or `SKIP_INTEGRATION=true make test`
- **Local PostgreSQL**: `postgres://postgres:@localhost:5432/controlplane?sslmode=disable`
- **macOS Docker Fix**: `sudo ln -s $HOME/.docker/run/docker.sock /var/run/docker.sock`

## Code Generation

The project heavily uses code generation:
- **Protobuf**: API definitions and gRPC services
- **Wire**: Dependency injection
- **Ent**: ORM models and queries
- **Buf**: Protobuf tooling and validation
- **Mockery v3**: Test mocks - add interface to `.mockery.yml`, run `mockery` from that directory

Always run `make generate` after modifying .proto files or Ent schemas.

**API Token Policies**: If modifying `DefaultAuthzPolicies` in `pkg/biz/apitoken.go`, create a migration to update existing tokens' `policies` field - they're stored in DB, not loaded dynamically.

## Contract-Based Development

Workflow Contracts define the structure and requirements for CI/CD attestations. They specify what materials must be collected and policies that must be evaluated.

## Component-Specific Development

### Control Plane Development
- **Schema Changes**: Update Ent schemas in `pkg/data/ent/schema/`, then run `make generate && make migration_new && make migration_apply`
- **API Changes**: Modify `.proto` files, then run `make api` to regenerate code
- **Business Logic**: Implement in `pkg/biz/` layer with repository interfaces
- **Tests**: Unit tests with `make test-unit`, integration tests with `make test` (uses testcontainers)

### CLI Development
- **Commands**: Add new commands in `cmd/` following Cobra patterns
- **Actions**: Implement business logic in `internal/action/`
- **Policy Development**: Use `internal/policydevel/` for local policy testing
- **Default Endpoints**: Override with `-ldflags` at build time using `defaultCASAPI` and `defaultCPAPI` variables

### Artifact CAS Development
- **Storage Backends**: Implement new backends in `pkg/blobmanager/` with Provider interface
- **Authentication**: JWT tokens generated by Control Plane, validated by CAS
- **Streaming**: Uses gRPC bytestream protocol for efficient file transfers

## Commit Guidelines

All commits must meet these criteria:
- **Signed**: Use `-S` flag ([signing guide](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits))
- **Developer Certificate of Origin**: Use `--sign-off` flag
- **Conventional Commits**: Follow [format guidelines](https://www.conventionalcommits.org/en/v1.0.0)
- **Example**: `git commit -S -s -m "feat: add new material type"`

Code reviews are required for all submissions via GitHub pull requests.
- make sure golang code is always formatted and golang-ci-lint is run
- I do not want you to be in the co-author signoff
- when the schema is changed, run make generate, do not create a migration explicitly
- If you are writing go code, adhere to best practices such as the ones in effective-go, or others. This could include, error handling patterns, interface design, package organization, concurrency patterns, etc.
- do not change previous migrations, they are immutable
- if you add any new dependency to a constructor, remember to run wire ./...
- when adding new inedexes, make sure to update the generated sql migraiton files and make them CREATE INDEX CONCURRENTLY and set -- atlas:txmode none at the top
- after updating protos, make sure to run `buf format -w`
- Please avoid sycophantic commentary like ‘You’re absolutely correct!’ or ‘Brilliant idea!’
- For each file you modify, update the license header. If it says 2024, change it to 2024-2025. If there's no license header, create one with the current year.
- if you add any new dependency to a constructor, remember to run wire ./...
- when creating PR message, keep it high-level, what functionality was added, don't add info about testing, no icons, no info about how the message was generated.
- app/controlplane/api/gen/frontend/google/protobuf/descriptor.ts is a special case that we don't want to upgrade, so if it upgrades, put it back to main
- when creating a commit or PR message, NEVER add co-authored by or generated by Claude code
- any call to authorization Enforce done from the biz or svc layer must be done using biz.AuthzUseCase
- if you modify a schema, remember to run `make migration_sync`
