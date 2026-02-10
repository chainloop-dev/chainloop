# SBOM Validation Policy Example

A policy that validates CycloneDX Software Bill of Materials (SBOMs) using the Chainloop JavaScript SDK.

## What This Policy Does

Validates CycloneDX SBOMs with these rules:

1. **Format validation**: Must be a CycloneDX format SBOM
2. **Component presence**: Must contain at least one component
3. **Name validation**: Each component must have a non-empty name
4. **Version validation**: Each component must have a non-empty version

This ensures SBOMs meet minimum completeness standards for supply chain tracking.

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

### Test with included SBOM:

```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material sbom.json \
  --kind SBOM_CYCLONEDX_JSON
```

Expected: Identifies components missing version information

### Test with valid SBOM:

```bash
cat > /tmp/valid-sbom.json << 'EOF'
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.4",
  "version": 1,
  "components": [
    {
      "name": "express",
      "version": "4.18.2",
      "type": "library"
    },
    {
      "name": "lodash",
      "version": "4.17.21",
      "type": "library"
    }
  ]
}
EOF

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/valid-sbom.json \
  --kind SBOM_CYCLONEDX_JSON
```

Expected: **PASS** (no violations)

### Test with missing component names:

```bash
cat > /tmp/missing-name-sbom.json << 'EOF'
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.4",
  "version": 1,
  "components": [
    {
      "name": "",
      "version": "1.0.0",
      "type": "library"
    }
  ]
}
EOF

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/missing-name-sbom.json \
  --kind SBOM_CYCLONEDX_JSON
```

Expected: **FAIL** with violation "component at index 0 missing name"

### Test with missing versions:

```bash
cat > /tmp/missing-version-sbom.json << 'EOF'
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.4",
  "version": 1,
  "components": [
    {
      "name": "my-package",
      "version": "",
      "type": "library"
    }
  ]
}
EOF

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/missing-version-sbom.json \
  --kind SBOM_CYCLONEDX_JSON
```

Expected: **FAIL** with violation "component 'my-package' missing version"

### Test with empty SBOM:

```bash
cat > /tmp/empty-sbom.json << 'EOF'
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.4",
  "version": 1,
  "components": []
}
EOF

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/empty-sbom.json \
  --kind SBOM_CYCLONEDX_JSON
```

Expected: **FAIL** with violation "SBOM must contain at least one component"

### Test with non-CycloneDX format:

```bash
cat > /tmp/wrong-format.json << 'EOF'
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0"
}
EOF

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/wrong-format.json \
  --kind SBOM_CYCLONEDX_JSON
```

Expected: **SKIP** with reason "not a CycloneDX SBOM"

## Key Concepts Demonstrated

1. **CycloneDX parsing**: Working with CycloneDX SBOM structure
2. **Component iteration**: Validating each component in the SBOM
3. **Multiple validation rules**: Checking both name and version fields
4. **Index-based errors**: Providing specific component locations in error messages
5. **Early return**: Skipping validation if format doesn't match
6. **Logging progress**: Showing component count and validation results

## SDK Functions Used

- `getMaterialJSON()` - Parse CycloneDX SBOM from material
- `success()` - Create success result
- `skip()` - Skip non-CycloneDX materials
- `outputResult()` - Output validation result
- `result.addViolation()` - Add violation for missing fields
- `result.hasViolations()` - Check if validation failed
- `logInfo()` - Log component count and results
- `logError()` - Log validation failures
- `run()` - Execution wrapper with error handling

## Understanding CycloneDX Structure

CycloneDX SBOMs follow this basic structure:

```json
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.4",
  "version": 1,
  "metadata": {
    "component": {
      "name": "my-application",
      "version": "1.0.0"
    }
  },
  "components": [
    {
      "name": "dependency-name",
      "version": "1.2.3",
      "type": "library",
      "purl": "pkg:npm/dependency-name@1.2.3",
      "licenses": [...]
    }
  ]
}
```

This policy validates the `components` array to ensure completeness.

## Files

- `policy.js` - Main policy implementation
- `policy.d.ts` - TypeScript type definitions for WASM exports
- `policy.yaml` - Policy configuration
- `sbom.json` - Sample SBOM data for testing
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
    - type: SBOM_CYCLONEDX_JSON
      name: application-sbom

  policies:
    materials:
      - ref: ./sbom/policy.yaml
        selector:
          name: application-sbom
```

## Real-World Usage

This policy ensures that all SBOMs in your supply chain:
1. Follow the CycloneDX standard
2. Include complete component information (name and version)
3. Meet minimum quality standards for dependency tracking

This is essential for:
- **Vulnerability Management**: Identify which versions are in use
- **License Compliance**: Track component licenses for legal review
- **Supply Chain Security**: Understand all dependencies in your software
- **Audit Requirements**: Provide complete inventory for auditors

## Extending the Policy

You can extend this policy to validate additional fields:

```javascript
// Validate license information
if (!comp.licenses || comp.licenses.length === 0) {
  result.addViolation(`component '${comp.name}' missing license information`);
}

// Validate package URL (purl)
if (!comp.purl) {
  result.addViolation(`component '${comp.name}' missing package URL (purl)`);
}

// Validate component type
const validTypes = ['application', 'library', 'framework', 'container', 'file'];
if (!validTypes.includes(comp.type)) {
  result.addViolation(`component '${comp.name}' has invalid type: ${comp.type}`);
}
```

## License

Apache License 2.0
