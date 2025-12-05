# SBOM Validation Policy Example

Validates CycloneDX SBOM structure and ensures components have required metadata.

## What This Policy Does

Validates a CycloneDX SBOM with the following rules:

**Hard Requirements** (violations):
1. Must be CycloneDX format (otherwise skips)
2. Must contain at least one component
3. Each component must have a name
4. Each component must have a version

**Soft Requirements** (warnings only):
- Components should have purl (package URL)
- Components should have integrity hashes
- SBOM should have timestamp
- SBOM should list generation tools

## Building

```bash
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
```

## Testing

### Valid SBOM:
```bash
cat > sbom.json <<EOF
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.4",
  "metadata": {
    "timestamp": "2024-01-01T00:00:00Z",
    "tools": [{"name": "syft", "version": "0.95.0"}]
  },
  "components": [
    {
      "type": "library",
      "name": "lodash",
      "version": "4.17.21",
      "purl": "pkg:npm/lodash@4.17.21",
      "hashes": [{"alg": "SHA-256", "content": "abc123..."}]
    }
  ]
}
EOF

chainloop policy develop eval \
  --policy policy.yaml \
  --material sbom.json \
  --kind SBOM_CYCLONEDX_JSON
```

Expected: No violations

### Invalid SBOM (missing component version):
```bash
cat > sbom-invalid.json <<EOF
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.4",
  "components": [
    {
      "type": "library",
      "name": "lodash"
    }
  ]
}
EOF

chainloop policy develop eval \
  --policy policy.yaml \
  --material sbom-invalid.json \
  --kind SBOM_CYCLONEDX_JSON
```

Expected: Violation: "component 'lodash' missing version"

### Non-CycloneDX SBOM:
```bash
cat > sbom-spdx.json <<EOF
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0"
}
EOF

chainloop policy develop eval \
  --policy policy.yaml \
  --material sbom-spdx.json \
  --kind SBOM_CYCLONEDX_JSON
```

Expected: Skipped: "not a CycloneDX SBOM"

## Key Concepts Demonstrated

1. **Structured input parsing**: Working with complex JSON structures
2. **Collection iteration**: Validating multiple components
3. **Skip conditions**: When policy doesn't apply to input
4. **Warning logs**: Non-critical issues (LogWarn vs violations)
5. **Incremental violations**: Building up violations during iteration

## File Structure

```
sbom/
├── policy.go       # Policy implementation
├── policy.yaml     # Policy specification
├── policy.wasm     # Compiled WASM module (after build)
└── README.md       # This file
```

## Real-World Usage

This policy can be used to enforce SBOM quality standards:

```yaml
# workflow-contract.yaml
spec:
  policies:
    - ref: file://./sbom-validation-policy.yaml
      with:
        name: sbom-quality-check

  materials:
    - type: SBOM_CYCLONEDX_JSON
      name: application-sbom
      output: true
```

The policy will run automatically when SBOMs are added to attestations.
