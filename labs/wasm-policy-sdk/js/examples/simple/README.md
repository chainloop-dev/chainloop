# Simple JavaScript Policy Example

A basic policy that validates string input using the Chainloop JavaScript SDK.

## What This Policy Does

This policy validates a message field in JSON input and checks:
1. Message is not empty
2. Message doesn't contain forbidden words (forbidden, banned, prohibited)
3. Message length is within configured maximum (default: 100 characters)

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

### Test 1: Valid Message

```bash
echo '{"message": "hello world"}' > /tmp/test-data.json

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/test-data.json \
  --kind STRING
```

Expected: **PASS** (no violations)

### Test 2: Empty Message

```bash
echo '{"message": ""}' > /tmp/test-data.json

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/test-data.json \
  --kind STRING
```

Expected: **FAIL** with violation "message cannot be empty"

### Test 3: Forbidden Word

```bash
echo '{"message": "this is forbidden content"}' > /tmp/test-data.json

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/test-data.json \
  --kind STRING
```

Expected: **FAIL** with violation "message contains forbidden word: forbidden"

### Test 4: Message Too Long

```bash
echo '{"message": "This is a very long message that exceeds the maximum allowed length of one hundred characters and should trigger a validation error"}' > /tmp/test-data.json

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/test-data.json \
  --kind STRING
```

Expected: **FAIL** with violation "message too long: 151 characters (max 100)"

### Test 5: Custom Max Length

```bash
echo '{"message": "short"}' > /tmp/test-data.json

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/test-data.json \
  --kind STRING \
  --input max_length=3
```

Expected: **FAIL** with violation "message too long: 5 characters (max 3)"

### Test 6: Missing Message Field

```bash
echo '{"data": "something"}' > /tmp/test-data.json

chainloop policy develop eval \
  --policy policy.yaml \
  --material /tmp/test-data.json \
  --kind STRING
```

Expected: **SKIP** with reason "Material missing 'message' field"

## Debug Logging

Run with `--debug` to see detailed logs:

```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material test-data.json \
  --kind STRING \
  --debug
```

Output will include:
```
INF Validating message with max length: 100
INF Validation passed for message: hello world
```

## Policy Arguments

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_length` | integer | `100` | Maximum allowed message length |

Pass arguments using `--input`:

```bash
--input max_length=50
```

## Files

- `policy.js` - Main policy implementation
- `policy.d.ts` - TypeScript type definitions for WASM exports
- `policy.yaml` - Policy configuration
- `test-data.json` - Sample test data
- `README.md` - This file

## SDK Functions Used

- `getMaterialJSON()` - Parse JSON material
- `getArgs()` - Get policy arguments
- `getArgStringDefault()` - Get argument with default
- `success()` - Create success result
- `skip()` - Create skip result
- `outputResult()` - Output result
- `logInfo()` - Log information
- `logError()` - Log errors
- `run()` - Execution wrapper with error handling

## Integration with Chainloop

Add to workflow contract:

```yaml
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: secure-build
spec:
  materials:
    - type: STRING
      name: commit-message

  policies:
    materials:
      - ref: ./simple/policy.yaml
        selector:
          name: commit-message
        with:
          max_length: "100"
```

## License

Apache License 2.0
