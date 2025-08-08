# SBOM Freshness

Validates that SBOM timestamps are within an acceptable age limit to ensure software component information is current.

## What This Policy Does

This policy validates the freshness of SBOM (Software Bill of Materials) files by checking their creation timestamp. It can:

- **Timestamp validation** - Verify SBOM was created within specified time limit
- **Configurable age limits** - Set custom freshness requirements (default: 30 days)  
- **Missing timestamp detection** - Ensure required timestamp field exists
- **CycloneDX format support** - Works with CycloneDX SBOM files

## Policy Parameters

| Parameter | Description | Required | Default | Example Values |
|-----------|-------------|----------|---------|----------------|
| `freshness_days` | Maximum age for SBOM in days | ❌ No | 30 | `7`, `30`, `90` |

## Using in Workflow Contracts

Add this policy to your workflow contract:

```yaml
apiVersion: workflowcontract.chainloop.dev/v1
kind: WorkflowContract
metadata:
  name: my-workflow
spec:
  materials:
    - type: SBOM_CYCLONEDX_JSON
      name: app-sbom
      
  policies:
    - ref: ./sbom-freshness/policy.yaml
      with:
        freshness_days: 30
```

### Multiple Freshness Requirements

```yaml
policies:
  # Strict freshness for production
  - ref: ./sbom-freshness/policy.yaml  
    with:
      freshness_days: 7
      
  # Moderate freshness for development
  - ref: ./sbom-freshness/policy.yaml
    with:
      freshness_days: 30
```

## Development & Testing

### Lint the Policy
```bash
chainloop policy develop lint --policy policy.yaml --format
```

### Manual Testing
```bash
# Test with fresh SBOM (should pass)
chainloop policy develop eval \
  --policy policy.yaml \
  --material testdata/sbom-fresh.json \
  --kind SBOM_CYCLONEDX_JSON

# Test with old SBOM (should fail)
chainloop policy develop eval \
  --policy policy.yaml \
  --material testdata/sbom-old.json \
  --kind SBOM_CYCLONEDX_JSON

# Test with custom freshness limit
chainloop policy develop eval \
  --policy policy.yaml \
  --material testdata/sbom-old.json \
  --kind SBOM_CYCLONEDX_JSON \
  --input freshness_days=400
```

### Run All Tests
```bash
./test.sh
```

## SBOM Timestamp Format

The policy expects CycloneDX SBOMs with RFC3339 timestamps in the metadata:

```json
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "metadata": {
    "timestamp": "2025-07-27T10:30:00Z",
    "tools": [...]
  },
  "components": [...]
}
```

## Freshness Calculation

The policy calculates freshness using:

1. **Current time** - `time.now_ns()` in nanoseconds
2. **SBOM timestamp** - Parsed from `metadata.timestamp` field
3. **Age limit** - `freshness_days * 24 * 60 * 60 * 1000 * 1000 * 1000` nanoseconds
4. **Validation** - Fails if `(current_time - sbom_time) > age_limit`

## Common Use Cases

### Security Compliance
```yaml
# Require fresh SBOMs for security scanning
policies:
  - ref: ./sbom-freshness/policy.yaml
    with:
      freshness_days: 7  # Weekly refresh requirement
```

### Development Workflows
```yaml
# Allow older SBOMs for development environments
policies:
  - ref: ./sbom-freshness/policy.yaml
    with:
      freshness_days: 90  # Quarterly refresh acceptable
```

### Release Gates
```yaml
# Strict freshness for production releases
policies:
  - ref: ./sbom-freshness/policy.yaml
    with:
      freshness_days: 1  # Daily refresh for releases
```

## Test Framework

- `_testutils.sh` - Contains shared test logic and utilities
- `test.sh` - Policy-specific test cases for various scenarios

## Sample Test Data

The `testdata/` directory contains:
- `sbom-fresh.json` - Recent SBOM (should pass default 30-day limit)
- `sbom-old.json` - Old SBOM from 2024 (should fail default limit)
- `sbom-missing-timestamp.json` - SBOM without timestamp (should fail)