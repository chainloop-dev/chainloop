# Attestation Policy Example

A policy that validates in-toto attestations for git commits using the Chainloop JavaScript SDK.

## What This Policy Does

Validates in-toto attestations with these rules:

1. **Attestation type check**: Must be a valid in-toto Statement (v0.1 or v1)
2. **Subject validation**: Must contain at least one subject
3. **Git commit validation**: Must reference a git commit (git.head)
4. **SHA1 digest validation**: Git commit must have a valid SHA1 digest (40 hex characters)
5. **Predicate type**: Must have a non-empty predicateType

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

### Test with valid attestation:

```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material attestation.json \
  --kind ATTESTATION
```

Expected: **PASS** (no violations)

### Test with custom attestation:

```bash
# Valid attestation with git commit
cat > /tmp/test-attestation.json << 'EOF'
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    {
      "name": "git.head",
      "digest": {
        "sha1": "abc123def456789012345678901234567890abcd"
      }
    }
  ],
  "predicateType": "https://slsa.dev/provenance/v1"
}
EOF

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/test-attestation.json \
  --kind ATTESTATION
```

Expected: **PASS**

### Test invalid cases:

```bash
# Missing git.head subject
cat > /tmp/test-no-git.json << 'EOF'
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    {
      "name": "container-image",
      "digest": {
        "sha256": "abc123"
      }
    }
  ],
  "predicateType": "https://slsa.dev/provenance/v1"
}
EOF

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/test-no-git.json \
  --kind ATTESTATION
```

Expected: **FAIL** with violation "attestation must reference a git commit (git.head)"

```bash
# Invalid SHA1 digest length
cat > /tmp/test-bad-sha.json << 'EOF'
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    {
      "name": "git.head",
      "digest": {
        "sha1": "tooshort"
      }
    }
  ],
  "predicateType": "https://slsa.dev/provenance/v1"
}
EOF

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/test-bad-sha.json \
  --kind ATTESTATION
```

Expected: **FAIL** with violation "git.head sha1 digest has invalid length: 8 (expected 40)"

## Key Concepts Demonstrated

1. **In-toto attestation parsing**: Working with in-toto Statement format
2. **Nested structure validation**: Accessing subjects, digests, and metadata
3. **Type checking**: Validating _type field against known Statement versions
4. **Complex validation logic**: Multiple validation rules with specific error messages
5. **Hexadecimal validation**: Checking SHA1 format and character validity
6. **Debug logging**: Using `logDebug()` for detailed troubleshooting

## SDK Functions Used

- `getMaterialJSON()` - Parse in-toto attestation from material
- `success()` - Create success result
- `skip()` - Skip non-attestation materials
- `outputResult()` - Output validation result
- `result.addViolation()` - Add specific violation messages
- `logInfo()` - Log validation progress
- `logError()` - Log validation failures
- `logDebug()` - Log detailed debugging information
- `run()` - Execution wrapper with error handling

## Understanding In-Toto Attestations

In-toto attestations follow a standardized format:

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    {
      "name": "artifact-name",
      "digest": {
        "sha256": "...",
        "sha1": "..."
      }
    }
  ],
  "predicateType": "https://slsa.dev/provenance/v1",
  "predicate": {
    "buildType": "...",
    "builder": {...},
    "materials": [...]
  }
}
```

This policy focuses on validating that git commits are properly referenced in the subject array.

## Files

- `policy.js` - Main policy implementation
- `policy.d.ts` - TypeScript type definitions for WASM exports
- `policy.yaml` - Policy configuration
- `attestation.json` - Sample attestation data for testing
- `esbuild.js` - Build configuration
- `package.json` - npm package configuration
- `README.md` - This file

## Integration with Chainloop

Add to workflow contract:

```yaml
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: secure-build
spec:
  materials:
    - type: ATTESTATION
      name: build-attestation

  policies:
    materials:
      - ref: ./attestation/policy.yaml
        selector:
          name: build-attestation
```

## Real-World Usage

This policy ensures that all attestations in your supply chain:
1. Follow the in-toto Statement standard
2. Reference specific git commits with valid SHA1 digests
3. Include proper predicate types for provenance tracking

This is essential for:
- **Auditability**: Trace every artifact back to its source code commit
- **Compliance**: Meet regulatory requirements for software provenance
- **Security**: Prevent attestations without proper source references

## License

Apache License 2.0
