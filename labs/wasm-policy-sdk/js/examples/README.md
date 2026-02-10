# JavaScript Policy Examples

This directory contains example policies demonstrating various features of the Chainloop JavaScript SDK.

## Examples

### 1. Simple Message Validation (`simple/`)

Basic string validation example demonstrating:
- Material extraction with `getMaterialJSON()`
- Policy arguments with `getArgStringDefault()`
- Result building with `success()`, `skip()`, and violations
- Logging with `logInfo()`, `logError()`
- Error handling with `run()` wrapper

**Build:**
```bash
cd simple
npm install
npm run build
```

**Test:**
```bash
# Valid message
chainloop policy develop eval \
  --policy simple/policy.yaml \
  --material simple/test-data.json \
  --kind EVIDENCE

# Empty message (violation)
echo '{"message": ""}' > /tmp/test.json
chainloop policy develop eval \
  --policy simple/policy.yaml \
  --material /tmp/test.json \
  --kind EVIDENCE

# Forbidden word (violation)
echo '{"message": "this is forbidden"}' > /tmp/test.json
chainloop policy develop eval \
  --policy simple/policy.yaml \
  --material /tmp/test.json \
  --kind EVIDENCE

# Custom max length
echo '{"message": "short"}' > /tmp/test.json
chainloop policy develop eval \
  --policy simple/policy.yaml \
  --material /tmp/test.json \
  --kind EVIDENCE \
  --input max_length=3
```

### 2. SBOM Validation (`sbom/`)

CycloneDX SBOM validation example demonstrating:
- Parsing SBOM materials
- Validating component structure
- Checking for required fields
- Logging validation progress

**Build:**
```bash
cd sbom
npm install
npm run build
```

**Test:**
```bash
# Valid SBOM
chainloop policy develop eval \
  --policy sbom/policy.yaml \
  --material sbom/sbom.json \
  --kind SBOM_CYCLONEDX_JSON
```

### 3. HTTP Requests (`http/`)

HTTP request example demonstrating:
- Making external API calls with `httpGetJSON()`
- Hostname restrictions for security
- Error handling for network failures
- Validating API responses

**Build:**
```bash
cd http
npm install
npm run build
```

**Test:**
```bash
# With allowed hostname
chainloop policy develop eval \
  --policy http/policy.yaml \
  --material http/data.json \
  --kind EVIDENCE \
  --allowed-hostnames httpbin.org

# With blocked hostname (will fail)
chainloop policy develop eval \
  --policy http/policy.yaml \
  --material http/data.json \
  --kind EVIDENCE \
  --allowed-hostnames www.example.com
```

### 4. Attestation Validation (`attestation/`)

In-toto attestation validation example demonstrating:
- Parsing in-toto attestation materials
- Complex validation logic with nested structures
- Git commit SHA validation
- Predicate type checking
- Debug logging with `logDebug()`

**Build:**
```bash
cd attestation
npm install
npm run build
```

**Test:**
```bash
# Valid attestation
chainloop policy develop eval \
  --policy attestation/policy.yaml \
  --material attestation/attestation.json \
  --kind ATTESTATION
```

## Common Build Steps

Each example follows the same build process:

1. **Install dependencies:**
   ```bash
   npm install
   ```

2. **Build the policy:**
   ```bash
   npm run build
   ```

   This runs:
   - `node esbuild.js` - Bundles the policy with the SDK
   - `extism-js dist/policy.js -i policy.d.ts -o policy.wasm` - Compiles to WASM

3. **Test locally:**
   ```bash
   chainloop policy develop eval \
     --policy policy.yaml \
     --material <material-file> \
     --kind <material-kind>
   ```

## Project Structure

Each example contains:
- `policy.js` - Main policy implementation
- `policy.yaml` - Policy metadata and configuration
- `policy.d.ts` - TypeScript declarations for WASM exports
- `esbuild.js` - Build configuration for bundling
- `package.json` - npm package configuration
- Test data files (JSON)

## SDK Features Demonstrated

### Material Extraction
- `getMaterialJSON()` - Parse JSON material
- `getMaterialString()` - Get material as string
- `getMaterialBytes()` - Get material as bytes

### Arguments
- `getArgs()` - Get all policy arguments
- `getArgString(key)` - Get specific argument
- `getArgStringDefault(key, default)` - Get argument with default

### Results
- `success()` - Create success result
- `fail(...violations)` - Create failure with violations
- `skip(reason)` - Skip policy execution
- `result.addViolation(message)` - Add violation to result
- `outputResult(result)` - Output final result

### Logging
- `logInfo(message)` - Info level logging
- `logDebug(message)` - Debug level logging
- `logWarn(message)` - Warning level logging
- `logError(message)` - Error level logging

### HTTP Requests
- `httpGet(url)` - Perform GET request
- `httpGetJSON(url)` - GET request with JSON parsing
- `httpPost(url, body)` - Perform POST request
- `httpPostJSON(url, obj)` - POST request with JSON

### Execution
- `run(fn)` - Wrap policy execution with error handling

## Troubleshooting

### Build fails with "Command not found: extism-js"
Install the Extism JS compiler:
```bash
npm install -g @extism/extism-js
```

### Build fails with "Cannot find module"
Make sure to run `npm install` in the example directory first.

### Policy fails with "hostname not allowed"
Add the hostname to the `--allowed-hostnames` flag when testing HTTP policies.

### Test data format errors
Ensure test data matches the expected structure for the material kind. Check the example data files for reference.

## Next Steps

- Review the [JavaScript SDK README](../README.md) for detailed API documentation
- Check the [Go examples](../../go/examples/) for additional patterns
- Read the [Policy Development Guide](https://docs.chainloop.dev/policies/) for best practices
