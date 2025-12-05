# Discover Policy Example

A policy that checks for policy violations in related attestations using the discover builtin function.

## What This Policy Does

Validates that container images do not have related attestations with policy violations:
1. Extracts the digest from container image metadata (`chainloop_metadata.digest.sha256`)
2. Uses the Discover() function to explore the artifact graph
3. Checks all related attestations for policy violations
4. Reports violations with detailed metadata (name, project, organization)

This matches the logic of the reference Rego policy for the discover builtin.

## Building

```bash
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
```

## Testing

### Testing with container image:
```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material docker://nginx:latest \
  --kind CONTAINER_IMAGE
```

Expected: **PASS** if no related attestations have policy violations, **FAIL** otherwise

### Testing with local image:
```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material docker://myorg/myapp:v1.0.0 \
  --kind CONTAINER_IMAGE
```

## Prerequisites

- The policy engine must have a gRPC connection configured
- The digest must correspond to an artifact in your Chainloop organization
- You must have access permissions to view the artifact and its references

## Key Concepts Demonstrated

1. **Container image metadata**: Extracting digest from `chainloop_metadata.digest.sha256`
2. **Discover function**: `Discover(digest, kind)` to explore artifact graph
3. **Result processing**: Handling DiscoverResult with references
4. **Metadata access**: Reading attestation metadata (hasPolicyViolations, name, project, organization)
5. **Conditional logic**: Building violations only for attestations with violations
6. **Error handling**: Gracefully handling missing gRPC connections

## File Structure

```
discover/
├── policy.go       # Policy implementation with discover
├── policy.yaml     # Policy specification
├── policy.wasm     # Compiled WASM module (after build)
├── go.mod          # Go module definition
└── README.md       # This file
```

## How Discover Works

The discover function queries Chainloop's backend to find artifacts related to the input digest:

```go
// Discover all references
result, err := chainlooppolicy.Discover(digest, "")

// Or filter by kind
result, err := chainlooppolicy.Discover(digest, "ATTESTATION")
```

The result contains:
- `Digest`: The queried artifact digest
- `Kind`: The material type of the artifact
- `References`: Array of related artifacts with their metadata

Each reference includes:
- `Digest`: The referenced artifact digest
- `Kind`: The material type
- `Metadata`: Key-value pairs with additional information

Common metadata keys:
- `hasPolicyViolations`: "true" if the attestation has policy violations
- Additional fields depending on artifact type
