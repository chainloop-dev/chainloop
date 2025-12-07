# Discover JavaScript Policy Example

A policy that checks for policy violations in related attestations using the Chainloop discover builtin function.

## What This Policy Does

Validates that container images do not have related attestations with policy violations:
1. Extracts the digest from container image metadata (`chainloop_metadata.digest.sha256`)
2. Uses the `discover()` function to explore the artifact graph
3. Checks all related attestations for policy violations
4. Reports violations with detailed metadata (name, project, organization)

This matches the logic of the reference Rego policy for the discover builtin.

## Building

Install dependencies:

```bash
npm install
```

Build the policy:

```bash
npm run build
```

This will:
1. Use esbuild to bundle the policy with the Chainloop SDK
2. Use extism-js to compile the bundle to WASM

The build process creates:
- `dist/policy.js` - Bundled JavaScript (includes SDK)
- `policy.wasm` - Compiled WASM binary

## Testing

### Test with container image:

```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material docker://nginx:latest \
  --kind CONTAINER_IMAGE
```

Expected: **PASS** if no related attestations have policy violations, **FAIL** otherwise

### Test with local image:

```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material docker://myorg/myapp:v1.0.0 \
  --kind CONTAINER_IMAGE
```

### Test with debug logging:

```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material docker://nginx:latest \
  --kind CONTAINER_IMAGE \
  --debug
```

Output will include:
```
INF Discovering artifacts related to: sha256:abc123...
INF Found 3 references for artifact sha256:abc123...
INF Checking attestation: sha256:def456...
INF Validation passed: no related attestations have policy violations
```

## Prerequisites

- The policy engine must have a gRPC connection configured
- The digest must correspond to an artifact in your Chainloop organization
- You must have access permissions to view the artifact and its references

## Key Concepts Demonstrated

1. **Container image metadata**: Extracting digest from `chainloop_metadata.digest.sha256`
2. **Discover function**: `discover(digest, kind)` to explore artifact graph
3. **Result processing**: Handling discover results with references array
4. **Metadata access**: Reading attestation metadata (hasPolicyViolations, name, project, organization)
5. **Conditional logic**: Building violations only for attestations with violations
6. **Error handling**: Gracefully handling missing gRPC connections

## Files

- `policy.js` - Main policy implementation with discover
- `policy.d.ts` - TypeScript type definitions for WASM exports
- `policy.yaml` - Policy configuration
- `esbuild.js` - Build configuration
- `package.json` - NPM dependencies and scripts
- `README.md` - This file

## How Discover Works

The discover function queries Chainloop's backend to find artifacts related to the input digest:

```javascript
// Discover all references
const result = discover(digest, "");

// Or filter by kind
const result = discover(digest, "ATTESTATION");
```

The result contains:
- `digest`: The queried artifact digest
- `kind`: The material type of the artifact
- `references`: Array of related artifacts with their metadata

Each reference includes:
- `digest`: The referenced artifact digest
- `kind`: The material type
- `metadata`: Key-value pairs with additional information

Common metadata keys:
- `hasPolicyViolations`: "true" if the attestation has policy violations
- Additional fields depending on artifact type

## SDK Functions Used

- `getMaterialJSON()` - Parse JSON material
- `discover()` - Query artifact graph
- `success()` - Create success result
- `skip()` - Create skip result
- `outputResult()` - Output result
- `logInfo()` - Log information
- `logWarn()` - Log warnings
- `logError()` - Log errors
- `run()` - Execution wrapper with error handling

## Integration with Chainloop

Add to workflow contract to validate container images:

```yaml
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: secure-build
spec:
  materials:
    - type: CONTAINER_IMAGE
      name: app-image

  policies:
    materials:
      - ref: ./discover/policy.yaml
        selector:
          name: app-image
```

This policy will check that the container image doesn't have related attestations with policy violations.

## License

Apache License 2.0
