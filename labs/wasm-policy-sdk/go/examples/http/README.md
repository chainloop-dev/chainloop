# HTTP Example Policy

Demonstrates how policies can make HTTP requests to external APIs with hostname restrictions enforced by the policy engine.

## What This Policy Does

This policy demonstrates HTTP functionality by:
- Making an HTTP GET request to httpbin.org
- Logging the request progress
- Returning success if the request completes
- Failing if the hostname is not in the allowed list

## Security

HTTP requests are restricted by the policy engine's AllowedHosts configuration (Extism native feature). Only explicitly allowed hostnames can be accessed. Requests to non-allowed hostnames will fail.

## Building

```bash
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
```

## Testing

### Test with allowed hostname (success)

```bash
echo "test" | chainloop policy develop eval \
  --policy policy.yaml \
  --material - \
  --kind STRING \
  --allowed-hostnames httpbin.org
```

Expected output: Policy passes (no violations)

### Test with blocked hostname (failure)

```bash
echo "test" | chainloop policy develop eval \
  --policy policy.yaml \
  --material - \
  --kind STRING \
  --allowed-hostnames www.example.com
```

Expected output: Policy execution fails with "HTTP request not allowed"

## SDK Functions Used

- `chainlooppolicy.Run()` - Wraps policy execution
- `chainlooppolicy.HTTPGet()` - Makes HTTP GET request
- `chainlooppolicy.LogInfo()` - Logs informational messages
- `chainlooppolicy.LogError()` - Logs error messages
- `chainlooppolicy.Success()` - Creates success result
- `chainlooppolicy.Fail()` - Creates failure result
- `chainlooppolicy.OutputResult()` - Outputs result to host
