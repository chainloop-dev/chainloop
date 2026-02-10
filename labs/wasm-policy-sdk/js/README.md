# Chainloop JavaScript SDK for WASM Policies

A JavaScript/TypeScript SDK for writing Chainloop policies that compile to WebAssembly with Extism.

## Features

- Easy to use API with minimal boilerplate
- Works with both JavaScript and TypeScript
- Compiled WASM files (~2MB)
- Built-in HTTP support with hostname restrictions
- Comprehensive logging support
- Module-based architecture with esbuild bundling
- Tested with real-world examples

## Documentation

**Official Documentation:**
- [WASM Policies Overview](https://docs.chainloop.dev/guides/wasm-policies/overview)
- [JavaScript SDK Guide](https://docs.chainloop.dev/guides/wasm-policies/javascript-sdk)
- [Go SDK Guide](https://docs.chainloop.dev/guides/wasm-policies/go-sdk)

## Quick Start

### 1. Create a Policy

```javascript
const {
  getMaterialJSON,
  success,
  skip,
  outputResult,
  logInfo,
  run
} = require('@chainloop-dev/policy-sdk');

function Execute() {
  return run(() => {
    const input = getMaterialJSON();

    if (!input.message) {
      outputResult(skip("Material missing 'message' field"));
      return;
    }

    const result = success();
    if (input.message === "") {
      result.addViolation("message cannot be empty");
    }

    outputResult(result);
  });
}

module.exports = { Execute };
```

### 2. Build with esbuild + extism-js

```bash
npm install
npm run build
```

### 3. Test Locally

```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material data.json \
  --kind STRING
```

## Common Patterns

### Required Fields Validation

```javascript
function validateRequiredFields(input) {
  const result = success();
  const required = ['name', 'version', 'license'];

  required.forEach(field => {
    if (!input[field] || input[field] === "") {
      result.addViolation(`${field} is required`);
    }
  });

  return result;
}
```

### Allowlist/Blocklist Validation

```javascript
function validateLicense(license, approved, forbidden) {
  const result = success();

  if (forbidden.includes(license)) {
    result.addViolation(`license ${license} is forbidden`);
  } else if (approved.length > 0 && !approved.includes(license)) {
    result.addViolation(`license ${license} not approved`);
  }

  return result;
}
```

### Nested Structure Validation

```javascript
function validateComponents(components) {
  const result = success();

  if (!components || components.length === 0) {
    result.addViolation("must contain at least one component");
  }

  components.forEach((comp, i) => {
    if (!comp.name) result.addViolation(`component ${i} missing name`);
    if (!comp.version) result.addViolation(`component ${i} missing version`);
  });

  return result;
}
```

### External API Validation

```javascript
function validateWithAPI(name, version) {
  try {
    const data = httpGetJSON(`https://registry.npmjs.org/${name}`);
    const result = success();

    if (!data.versions[version]) {
      result.addViolation(`version ${version} not found`);
    }

    return result;
  } catch (e) {
    return skip(`API unavailable: ${e.message}`);
  }
}
```

## API Reference

### Execution

#### `run(policyFn)`

Wraps policy execution and handles errors gracefully.

```javascript
function Execute() {
  return run(() => {
    // Your policy logic here
  });
}
```

### Material Extraction

#### `getMaterialJSON()`

Parses the material input as JSON and returns the parsed object.

```javascript
const input = getMaterialJSON();
console.log(input.message);
```

#### `getMaterialString()`

Returns the material as a string.

```javascript
const text = getMaterialString();
```

#### `getMaterialBytes()`

Returns the material as raw bytes (Uint8Array).

```javascript
const data = getMaterialBytes();
```

### Arguments Extraction

#### `getArgs()`

Extracts all policy arguments passed via the engine configuration.

```javascript
const args = getArgs();
const maxLength = args.max_length;
```

#### `getArgString(key)`

Gets a specific argument value by key.

```javascript
const version = getArgString("expected_version");
```

#### `getArgStringDefault(key, defaultValue)`

Gets an argument value with a default fallback.

```javascript
const maxLength = getArgStringDefault("max_length", "100");
```

### Result Building

#### `success()`

Creates a success result with no violations.

```javascript
const result = success();
outputResult(result);
```

#### `fail(...violations)`

Creates a failure result with one or more violations.

```javascript
const result = fail("validation failed", "missing required field");
outputResult(result);
```

#### `skip(reason)`

Creates a skip result with a reason.

```javascript
const result = skip("not applicable for this material type");
outputResult(result);
```

### Result Methods

#### `addViolation(msg)`

Adds a violation message to a result.

```javascript
const result = success();
if (components.length === 0) {
  result.addViolation("SBOM must contain at least one component");
}
```

#### `hasViolations()`

Returns true if the result has any violations.

```javascript
if (result.hasViolations()) {
  logError(`Policy failed with ${result.violations.length} violations`);
}
```

#### `isSuccess()`

Returns true if the result has no violations and is not skipped.

```javascript
if (result.isSuccess()) {
  logInfo("Policy passed");
}
```

### Output

#### `outputResult(result)`

Outputs the result to the WASM host.

```javascript
const result = success();
outputResult(result);
```

### Logging

#### `logInfo(message)`

Logs an informational message.

```javascript
logInfo(`Processing ${components.length} components`);
```

#### `logDebug(message)`

Logs a debug message (visible with --debug flag).

```javascript
logDebug("Component details: " + JSON.stringify(component));
```

#### `logWarn(message)`

Logs a warning message.

```javascript
logWarn("Optional field missing: " + fieldName);
```

#### `logError(message)`

Logs an error message.

```javascript
logError("Validation failed: " + error.message);
```

### HTTP Requests

HTTP requests are restricted to hostnames configured in the policy engine's AllowedHosts. Requests to non-allowed hostnames will fail.

#### `httpGet(url)`

Performs an HTTP GET request and returns the response object.

```javascript
const response = httpGet("https://api.example.com/data");
if (response.status === 200) {
  console.log(response.body);
}
```

#### `httpGetJSON(url)`

Performs an HTTP GET request and parses the JSON response.

```javascript
const data = httpGetJSON("https://registry.npmjs.org/lodash");
console.log(`Package: ${data.name}`);
```

#### `httpPost(url, body)`

Performs an HTTP POST request.

```javascript
const response = httpPost("https://api.example.com/validate", jsonData);
```

#### `httpPostJSON(url, requestBody)`

Performs an HTTP POST request with JSON body and response.

```javascript
const result = httpPostJSON("https://api.example.com/validate", {data: "test"});
```

### Artifact Discovery

Explore the artifact graph to discover related artifacts. Requires a gRPC connection configured in the policy engine.

#### `discover(digest, kind)`

Discovers artifacts related to the given digest, optionally filtering by kind.

```javascript
// Discover all references for a container image
const digest = "sha256:abc123...";
const result = discover(digest);

// Check if any referenced attestations have policy violations
for (const ref of result.references) {
  if (ref.kind === "ATTESTATION") {
    if (ref.metadata.hasPolicyViolations === "true") {
      logWarn(`Attestation ${ref.digest} has policy violations`);
    }
  }
}

// Discover with kind filter
const attestations = discover(digest, "ATTESTATION");
```

**Parameters:**
- `digest` (string): The artifact digest to discover (e.g., "sha256:abc123...")
- `kind` (string, optional): Filter by material kind (e.g., "CONTAINER_IMAGE", "ATTESTATION")

**Returns:**
```typescript
{
  digest: string,
  kind: string,
  references: [
    {
      digest: string,
      kind: string,
      metadata: { [key: string]: string }
    }
  ]
}
```

#### `discoverByDigest(digest)`

Convenience function that calls discover with no kind filter.

```javascript
const result = discoverByDigest("sha256:abc123...");
```

## Examples

Complete working examples are in the `examples/` directory:

### Simple String Validation

Location: `examples/simple/`

Validates message strings for forbidden words and length limits. Demonstrates argument parsing and basic validation.

**Build:**
```bash
cd examples/simple
npm install
npm run build
```

**Test:**
```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material test-data.json \
  --kind EVIDENCE
```

### SBOM Validation

Location: `examples/sbom/`

Validates CycloneDX SBOM for required components, licenses, and security requirements.

**Build:**
```bash
cd examples/sbom
npm install
npm run build
```

### Attestation Validation

Location: `examples/attestation/`

Validates in-toto attestations for required predicates, subjects, and materials.

**Build:**
```bash
cd examples/attestation
npm install
npm run build
```

### HTTP API Integration

Location: `examples/http/`

Demonstrates external API calls with hostname restrictions for package validation.

**Build:**
```bash
cd examples/http
npm install
npm run build
```

### Artifact Discovery

Location: `examples/discover/`

Explores the artifact graph to detect policy violations in related attestations. Demonstrates the discover builtin function.

**Build:**
```bash
cd examples/discover
npm install
npm run build
```

**Test:**
```bash
chainloop policy develop eval \
  --material ghcr.io/chainloop-dev/chainloop/control-plane:v1.57.0-amd64 \
  --policy policy.yaml
```

## Project Setup

### package.json

```json
{
  "name": "my-policy",
  "version": "1.0.0",
  "scripts": {
    "build": "node esbuild.js && extism-js dist/policy.js -i policy.d.ts -o policy.wasm"
  },
  "devDependencies": {
    "esbuild": "^0.19.0"
  }
}
```

### esbuild.js

```javascript
const esbuild = require('esbuild');

esbuild.build({
  entryPoints: ['policy.js'],
  outdir: 'dist',
  bundle: true,
  sourcemap: false,
  minify: false,
  format: 'cjs',
  target: ['es2020'],
  platform: 'node',
  external: []
}).catch(() => process.exit(1));
```

### policy.d.ts

```typescript
declare module "main" {
  export function Execute(): I32;
}
```

### policy.yaml

```yaml
apiVersion: workflowcontract.chainloop.dev/v1
kind: Policy
metadata:
  name: my-policy
  description: My policy description
spec:
  policies:
    - kind: EVIDENCE  # or SBOM_CYCLONEDX_JSON, ATTESTATION, etc.
      path: policy.wasm
```

## JavaScript/TypeScript Compatibility

This SDK is designed to work within the Extism JS PDK's limitations.

### Supported Features

- ES2020 syntax (no ES2021+ features)
- CommonJS modules (require/module.exports)
- Simple objects and arrays
- JSON.parse/JSON.stringify
- Synchronous operations only
- String manipulation and basic math

### Unsupported Features

- async/await (no Promise support)
- Node.js built-in modules (fs, path, http, etc.)
- ES modules (import/export)
- Browser APIs (fetch, localStorage, etc.)
- setTimeout/setInterval
- Symbols and WeakMaps

### Recommended Patterns

```javascript
// Good: Simple objects
const component = {
  name: "lodash",
  version: "4.17.21",
  hashes: [
    { algorithm: "sha256", value: "abc123..." }
  ]
};

// Good: Synchronous operations
const result = success();
components.forEach(comp => {
  if (!comp.name) {
    result.addViolation("Component missing name");
  }
});

// Avoid: Async operations
// async function validate() {  // Not supported
//   const data = await fetch(...);
// }

// Avoid: Node.js modules
// const fs = require('fs');  // Not available in WASM
```

## Build Configuration

### Required Build Steps

```bash
# 1. Bundle with esbuild (combines SDK + policy)
node esbuild.js

# 2. Compile to WASM with extism-js
extism-js dist/policy.js -i policy.d.ts -o policy.wasm
```

Or use the npm script:
```bash
npm run build
```

### Build Options

**esbuild configuration:**
- `bundle: true` - Combines all modules into one file
- `format: 'cjs'` - CommonJS format (required)
- `target: ['es2020']` - JavaScript compatibility level
- `platform: 'node'` - Node.js platform conventions

**extism-js options:**
- `-i policy.d.ts` - Type definitions for WASM exports
- `-o policy.wasm` - Output WASM file

### Typical File Sizes

- Bundled JavaScript: ~7KB (policy + SDK)
- Compiled WASM: ~2.1MB (includes QuickJS runtime)

## Testing

Test your policies locally before deployment:

### Quick Test

```bash
# Build policy
npm run build

# Test with material
chainloop policy develop eval \
  --policy policy.yaml \
  --material test-data.json \
  --kind EVIDENCE

# Test with arguments
chainloop policy develop eval \
  --policy policy.yaml \
  --material test-data.json \
  --kind EVIDENCE \
  --input max_length=100

# Test with debug output
chainloop policy develop eval \
  --policy policy.yaml \
  --material test-data.json \
  --kind EVIDENCE \
  --debug
```

### Test Cases

Create test files for different scenarios:

| Test Case | Expected | Violation |
|-----------|----------|-----------|
| Valid input | Success | - |
| Empty/missing fields | Fail | Field-specific message |
| Wrong format | Skip | Format not supported |
| API unavailable | Skip | API error message |

### Automated Testing

Create a `test.sh` script:

```bash
#!/bin/bash

# Build policy
npm run build

# Run tests
test_policy() {
  local name=$1
  local material=$2
  local expect=$3

  echo "Testing: $name"
  output=$(chainloop policy develop eval \
    --policy policy.yaml \
    --material "$material" \
    --kind EVIDENCE 2>&1)

  if echo "$output" | grep -q "$expect"; then
    echo "✓ PASSED"
  else
    echo "✗ FAILED"
    echo "$output"
  fi
}

# Test cases
test_policy "Valid input" "test-data/valid.json" "violations\": \[\]"
test_policy "Invalid input" "test-data/invalid.json" "message cannot be empty"
test_policy "Wrong format" "test-data/wrong.json" "skipped\": true"
```

Make the script executable:
```bash
chmod +x test.sh
./test.sh
```

## Troubleshooting

### Build Errors

**Error: Cannot find module '@chainloop-dev/policy-sdk'**
- Cause: SDK not in node_modules or incorrect path
- Fix: Use relative path `require('../../index.js')` or install SDK as npm package

**Error: esbuild not found**
- Cause: esbuild not installed
- Fix: Run `npm install` to install dependencies

**Error: extism-js command not found**
- Cause: extism-js not installed globally
- Fix: Run `npm install -g @extism/js-pdk`

### Runtime Errors

**Policy execution failed: Host.inputString is not defined**
- Cause: SDK functions called outside Execute function
- Fix: Only call SDK functions inside the Execute function or callbacks

**HTTP request blocked**
- Cause: Hostname not in allowed list
- Fix: Add hostname with `--allowed-hostnames` flag:
  ```bash
  --allowed-hostnames registry.npmjs.org,api.github.com
  ```

**JSON parse error**
- Cause: Material is not valid JSON
- Fix: Use try-catch or skip() for non-JSON materials:
  ```javascript
  try {
    const data = getMaterialJSON();
  } catch (e) {
    outputResult(skip("Material is not valid JSON"));
    return;
  }
  ```

**Wrong material kind**
- Check: policy.yaml kind matches data type
- Fix: Use correct kind (SBOM_CYCLONEDX_JSON, STRING, EVIDENCE, ATTESTATION, etc.)

## Best Practices

1. **Use the SDK module** - Import from SDK instead of copying functions
2. **Validate early** - Check input format before complex logic
3. **Return specific violations** - Clear messages help users fix issues
4. **Use skip for non-applicable cases** - Don't fail when not applicable
5. **Test with real data** - Use actual SBOMs/attestations for testing
6. **Log validation progress** - Use logging functions for debugging
7. **Handle errors gracefully** - Use try-catch for operations that might fail
8. **Keep it synchronous** - No async/await, all operations are synchronous

### Example: Error Handling

```javascript
function Execute() {
  return run(() => {
    // Parse with error handling
    let input;
    try {
      input = getMaterialJSON();
    } catch (e) {
      outputResult(skip("Invalid JSON: " + e.message));
      return;
    }

    // Validate with clear messages
    const result = success();

    if (!input.message) {
      outputResult(skip("Material missing 'message' field"));
      return;
    }

    if (input.message === "") {
      result.addViolation("message cannot be empty");
    }

    if (input.message.length > 100) {
      result.addViolation(`message too long: ${input.message.length} characters (max 100)`);
    }

    outputResult(result);
  });
}
```

## Installation

### Prerequisites

```bash
# Install extism-js globally
npm install -g @extism/js-pdk

# Verify installation
extism-js --version
```

### Project Setup

```bash
# Create project directory
mkdir my-policy
cd my-policy

# Initialize npm
npm init -y

# Install build dependencies
npm install --save-dev esbuild

# Copy SDK files (or clone from repo)
mkdir -p sdk
cp /path/to/chainloop/sdk/js/index.js sdk/
cp /path/to/chainloop/sdk/js/index.d.ts sdk/
```

## License

Apache License 2.0 - See LICENSE file for details.
