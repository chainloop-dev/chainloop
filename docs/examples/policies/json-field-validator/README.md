# JSON Field Validator

Validates specific fields in JSON files used as evidence in Chainloop workflows.

## What This Policy Does

This policy allows you to validate individual fields in JSON evidence files. It can:

- **Check field values** - Verify a field matches an expected value
- **Pattern matching** - Validate field values against regex patterns  
- **Structure validation** - Ensure required fields exist
- **Flexible comparison** - Supports various boolean formats (`true`, `True`, `1`)

## Policy Parameters

| Parameter | Description | Required | Example Values |
|-----------|-------------|----------|----------------|
| `required_field` | Dot-notation path to the field | ✅ Yes | `application.name`, `security.enabled` |
| `expected_value` | Expected exact value | ❌ No | `web-service`, `production`, `true` |
| `field_pattern` | Regex pattern to match | ❌ No | `^[0-9]+\.[0-9]+\.[0-9]+$` |

**Note**: Use either `expected_value` OR `field_pattern`, not both.

## Using in Workflow Contracts

Add this policy to your workflow contract:

```yaml
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: my-workflow
  description: Workflow contract for application config validation
spec:
  materials:
    - type: EVIDENCE
      name: app-config

  policies:
    materials:
      - ref: ./json-field-validator/policy.yaml
        with:
          required_field: application.environment
          expected_value: production
```

### Multiple Field Validations

```yaml
policies:
  materials:
    # Validate environment
    - ref: ./json-field-validator/policy.yaml
      with:
        required_field: application.environment
        expected_value: production

    # Validate version format
    - ref: ./json-field-validator/policy.yaml
      with:
        required_field: application.version
        field_pattern: "^[0-9]+\\.[0-9]+\\.[0-9]+$"

    # Validate security is enabled
    - ref: ./json-field-validator/policy.yaml
      with:
        required_field: security.enabled
        expected_value: "true"
```

## Development & Testing

### Lint the Policy
```bash
chainloop policy develop lint --policy policy.yaml --format
```

### Manual Testing
```bash
# Test field value validation
chainloop policy develop eval \
  --policy policy.yaml \
  --material testdata/config.json \
  --kind EVIDENCE \
  --input required_field=application.name \
  --input expected_value=web-service

# Test pattern validation  
chainloop policy develop eval \
  --policy policy.yaml \
  --material testdata/config.json \
  --kind EVIDENCE \
  --input required_field=application.version \
  --input field_pattern="^[0-9]+\\.[0-9]+\\.[0-9]+$"
```

### Run All Tests
```bash
./test.sh
```

## Supported Field Paths

The policy supports dot notation for nested fields:

```json
{
  "application": {
    "name": "web-service",           // → application.name  
    "version": "2.1.0",             // → application.version
    "environment": "production"     // → application.environment
  },
  "security": {
    "enabled": true,                // → security.enabled
    "tls_version": "1.3"           // → security.tls_version
  }
}
```

## Boolean Value Support

The policy accepts multiple boolean formats for `expected_value`:
- `true` (JSON boolean as string)
- `True` (capitalized)  
- `1` (numeric string)

All match against JSON `true` values.

## Test Framework

- `_testutils.sh` - Contains shared test logic and utilities
- `test.sh` - Policy-specific test cases

## Sample Test Data

The `testdata/` directory contains:
- `config.json` - Application configuration with nested structure
- `compliance-checklist.json` - Different JSON structure for negative testing
- `invalid.json` - Invalid JSON for error testing