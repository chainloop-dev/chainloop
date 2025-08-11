# HTTP Hostname Validation Example

Demonstrates how to make HTTP requests to external APIs from Chainloop policies while maintaining security through hostname allowlisting.

## What This Policy Does

This policy is just an example which validates the Chainloop platform version by making HTTP requests to its external info API.

It demonstrates:

- **HTTP API Integration** - Makes requests to `https://app.chainloop.dev/api/info`
- **Response validation** - Compares API response against expected version (configurable)
- **Hostname Security** - Requires explicit hostname allowlisting for HTTP requests
- **Error Handling** - Gracefully handles network failures and API errors

## Policy Parameters

| Parameter | Description | Required | Default | Example |
|-----------|-------------|----------|---------|---------|
| `expected_version` | Expected platform version to validate against | ❌ No | `1.2.3` | `v0.256.0`, `2.0.0` |

## The HTTP Security Challenge

By default, Chainloop policies **block all HTTP requests** for security reasons. This policy will fail with:

```
ERR evaluating policy: unallowed host: app.chainloop.dev
```

## Solution: Hostname Allowlisting

Use the `--allowed-hostnames` flag to explicitly allow specific hostnames:

```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material testdata/empty.json \
  --kind EVIDENCE \
  --allowed-hostnames app.chainloop.dev
```

## Using in Workflow Contracts

Add this policy to your workflow contract:

```yaml
apiVersion: workflowcontract.chainloop.dev/v1
kind: WorkflowContract
metadata:
  name: platform-validation-workflow
spec:
  materials:
    - type: EVIDENCE
      name: platform-check
      
  policies:
    - ref: ./http-hostname-validation/policy.yaml
      with:
        expected_version: "2.0.0"  # Optional, defaults to "1.2.3"
```

**Note**: When running in production, the Control Plane manages hostname allowlisting through organization settings. The `--allowed-hostnames` flag is only for local development and testing.

## Development & Testing

See [test.sh](test.sh) for the test cases.