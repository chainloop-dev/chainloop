# Chainloop Go SDK for WASM Policies

A Go SDK for writing Chainloop policies that compile to WebAssembly with TinyGo.

## Features

- Easy to use API with minimal boilerplate
- No generics required (TinyGo compatible)
- Type-safe material and argument extraction
- Built-in HTTP support with hostname restrictions
- Logging support
- Tested with real-world examples

## Documentation

**Official Documentation:**
- [WASM Policies Overview](https://docs.chainloop.dev/guides/wasm-policies/overview)
- [Go SDK Guide](https://docs.chainloop.dev/guides/wasm-policies/go-sdk)
- [JavaScript SDK Guide](https://docs.chainloop.dev/guides/wasm-policies/javascript-sdk)

## Quick Start

### 1. Create a Policy

```go
package main

import (
    chainlooppolicy "github.com/chainloop-dev/chainloop/labs/wasm-policy-sdk/go"
)

type Input struct {
    Message string `json:"message"`
}

//export Execute
func Execute() int32 {
    return chainlooppolicy.Run(func() {
        var input Input
        if err := chainlooppolicy.GetMaterialJSON(&input); err != nil {
            chainlooppolicy.OutputResult(chainlooppolicy.Skip(err.Error()))
            return
        }

        result := chainlooppolicy.Success()
        if input.Message == "" {
            result.AddViolation("message cannot be empty")
        }

        chainlooppolicy.OutputResult(result)
    })
}

func main() {}
```

### 2. Build with TinyGo

```bash
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
```

### 3. Test Locally

```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material data.json \
  --kind STRING
```

## API Reference

### Execution

#### `Run(policyFn func())`

Wraps policy execution and handles the WASM int32 return value.

```go
//export Execute
func Execute() int32 {
    return chainlooppolicy.Run(func() {
        // Your policy logic here
    })
}
```

### Material Extraction

#### `GetMaterialJSON(target interface{}) error`

Unmarshals the material input as JSON into the target struct.

```go
var input MyInput
if err := chainlooppolicy.GetMaterialJSON(&input); err != nil {
    chainlooppolicy.OutputResult(chainlooppolicy.Skip(err.Error()))
    return
}
```

#### `GetMaterialBytes() []byte`

Returns the raw material bytes from the input.

```go
data := chainlooppolicy.GetMaterialBytes()
```

#### `GetMaterialString() string`

Returns the material as a string.

```go
message := chainlooppolicy.GetMaterialString()
```

### Arguments Extraction

#### `GetArgs() (map[string]string, error)`

Extracts policy arguments passed via the engine configuration.

```go
args, err := chainlooppolicy.GetArgs()
if err != nil {
    // Handle error
}
maxLength := args["max_length"]
```

#### `GetArgString(key string) (string, error)`

Gets a specific argument value by key.

```go
value, err := chainlooppolicy.GetArgString("expected_version")
```

#### `GetArgStringDefault(key, defaultValue string) string`

Gets an argument value with a default fallback.

```go
maxLength := chainlooppolicy.GetArgStringDefault("max_length", "100")
```

### Result Building

#### `Success() Result`

Creates a success result with no violations.

```go
result := chainlooppolicy.Success()
chainlooppolicy.OutputResult(result)
```

#### `Fail(violations ...string) Result`

Creates a failure result with one or more violations.

```go
result := chainlooppolicy.Fail("validation failed", "missing required field")
chainlooppolicy.OutputResult(result)
```

#### `Skip(reason string) Result`

Creates a skip result with a reason.

```go
result := chainlooppolicy.Skip("not applicable for this material type")
chainlooppolicy.OutputResult(result)
```

#### `Skipf(format string, args ...interface{}) Result`

Creates a skip result with formatted reason.

```go
result := chainlooppolicy.Skipf("material version %s not supported", version)
chainlooppolicy.OutputResult(result)
```

### Result Methods

#### `AddViolation(msg string)`

Adds a violation message to a result.

```go
result := chainlooppolicy.Success()
if len(components) == 0 {
    result.AddViolation("SBOM must contain at least one component")
}
```

#### `AddViolationf(format string, args ...interface{})`

Adds a formatted violation message.

```go
result.AddViolationf("expected version %s, got %s", expected, actual)
```

#### `HasViolations() bool`

Returns true if the result has any violations.

```go
if result.HasViolations() {
    chainlooppolicy.LogError("Policy failed with %d violations", len(result.Violations))
}
```

#### `IsSuccess() bool`

Returns true if the result has no violations and is not skipped.

```go
if result.IsSuccess() {
    chainlooppolicy.LogInfo("Policy passed")
}
```

### Output

#### `OutputResult(result Result)`

Outputs the result to the WASM host.

```go
result := chainlooppolicy.Success()
chainlooppolicy.OutputResult(result)
```

### Logging

#### `LogInfo(format string, args ...interface{})`

Logs an informational message.

```go
chainlooppolicy.LogInfo("Processing %d components", len(components))
```

#### `LogDebug(format string, args ...interface{})`

Logs a debug message.

```go
chainlooppolicy.LogDebug("Component details: %+v", component)
```

#### `LogWarn(format string, args ...interface{})`

Logs a warning message.

```go
chainlooppolicy.LogWarn("Optional field missing: %s", fieldName)
```

#### `LogError(format string, args ...interface{})`

Logs an error message.

```go
chainlooppolicy.LogError("Validation failed: %v", err)
```

#### `LogTrace(format string, args ...interface{})`

Logs a trace message.

```go
chainlooppolicy.LogTrace("Entering validation function")
```

### HTTP Requests

HTTP requests are restricted to hostnames configured in the policy engine's AllowedHosts. Requests to non-allowed hostnames will fail.

#### `HTTPGet(url string) ([]byte, error)`

Performs an HTTP GET request and returns the response body.

```go
data, err := chainlooppolicy.HTTPGet("https://api.example.com/data")
if err != nil {
    // Handle error (hostname not allowed or request failed)
}
```

#### `HTTPGetJSON(url string, target interface{}) error`

Performs an HTTP GET request and unmarshals the JSON response.

```go
var response APIResponse
err := chainlooppolicy.HTTPGetJSON("https://api.example.com/data", &response)
```

#### `HTTPGetString(url string) (string, error)`

Performs an HTTP GET request and returns the response as a string.

```go
body, err := chainlooppolicy.HTTPGetString("https://api.example.com/data")
```

### Artifact Discovery

Explore the artifact graph to discover related artifacts. Requires a gRPC connection configured in the policy engine.

#### `Discover(digest, kind string) (*DiscoverResult, error)`

Discovers artifacts related to the given digest, optionally filtering by kind.

```go
// Discover all references for a container image
digest := "sha256:abc123..."
result, err := chainlooppolicy.Discover(digest, "")
if err != nil {
    // Handle error (no gRPC connection or discovery failed)
    return
}

// Check if any referenced attestations have policy violations
for _, ref := range result.References {
    if ref.Kind == "ATTESTATION" {
        if ref.Metadata["hasPolicyViolations"] == "true" {
            chainlooppolicy.LogWarn(fmt.Sprintf("Attestation %s has policy violations", ref.Digest))
        }
    }
}

// Discover with kind filter
attestations, err := chainlooppolicy.Discover(digest, "ATTESTATION")
```

**Parameters:**
- `digest` (string): The artifact digest to discover (e.g., "sha256:abc123...")
- `kind` (string): Optional filter by material kind (e.g., "CONTAINER_IMAGE", "ATTESTATION")

**Returns:**
```go
type DiscoverResult struct {
    Digest     string
    Kind       string
    References []Reference
}

type Reference struct {
    Digest   string
    Kind     string
    Metadata map[string]string
}
```

#### `DiscoverByDigest(digest string) (*DiscoverResult, error)`

Convenience function that calls Discover with no kind filter.

```go
result, err := chainlooppolicy.DiscoverByDigest("sha256:abc123...")
```

## Common Patterns

### Required Fields Validation

```go
func validateRequiredFields(comp Component) chainlooppolicy.Result {
  result := chainlooppolicy.Success()

  if comp.Name == "" {
    result.AddViolation("name is required")
  }
  if comp.Version == "" {
    result.AddViolation("version is required")
  }

  return result
}
```

### Allowlist/Blocklist Validation

```go
func validateLicense(license string, approved, forbidden []string) chainlooppolicy.Result {
  result := chainlooppolicy.Success()

  // Check blocklist first
  for _, blocked := range forbidden {
    if license == blocked {
      result.AddViolationf("%s is forbidden", license)
      return result
    }
  }

  // Check allowlist
  if len(approved) > 0 {
    found := false
    for _, allowed := range approved {
      if license == allowed {
        found = true
        break
      }
    }
    if !found {
      result.AddViolationf("%s not approved", license)
    }
  }

  return result
}
```

### Nested Structure Validation

```go
func validateComponents(components []Component) chainlooppolicy.Result {
  result := chainlooppolicy.Success()

  if len(components) == 0 {
    result.AddViolation("must contain at least one component")
  }

  for i, comp := range components {
    if comp.Name == "" {
      result.AddViolationf("component %d missing name", i)
    }
    if comp.Version == "" {
      result.AddViolationf("component %d missing version", i)
    }
  }

  return result
}
```

### External API Validation

```go
func validateWithAPI(name, version string) chainlooppolicy.Result {
  var data RegistryResponse
  err := chainlooppolicy.HTTPGetJSON(fmt.Sprintf("https://registry.npmjs.org/%s", name), &data)
  if err != nil {
    return chainlooppolicy.Skipf("API unavailable: %v", err)
  }

  result := chainlooppolicy.Success()
  if _, exists := data.Versions[version]; !exists {
    result.AddViolationf("version %s not found", version)
  }

  return result
}
```

## Examples

Complete working examples are in the `examples/` directory.

### Simple String Validation

Location: `examples/simple/`

**Build:**
```bash
cd examples/simple
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
```

**Test:**
```bash
chainloop policy develop eval \
  --policy policy.yaml \
  --material data.json \
  --kind STRING
```

### SBOM Validation

Location: `examples/sbom/`

Validates CycloneDX SBOMs for required component fields.

**Build:**
```bash
cd examples/sbom
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
```

### Attestation Validation

Location: `examples/attestation/`

Validates in-toto attestations for git commit subjects.

**Build:**
```bash
cd examples/attestation
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
```

### HTTP Requests

Location: `examples/http/`

Makes HTTP requests to external APIs with hostname restrictions.

**Build:**
```bash
cd examples/http
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
```

### Artifact Discovery

Location: `examples/discover/`

Explores the artifact graph to detect policy violations in related attestations.

**Build:**
```bash
cd examples/discover
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
```

**Test:**
```bash
chainloop policy develop eval \
  --material ghcr.io/chainloop-dev/chainloop/control-plane:v1.57.0-amd64 \
  --policy policy.yaml
```

## Project Setup

### go.mod

```go
module myorganization.com/chainloop-policies/my-policy

go 1.25

require (
    github.com/chainloop-dev/chainloop/labs/wasm-policy-sdk/go v0.0.0
    github.com/extism/go-pdk v1.1.3
)

replace github.com/chainloop-dev/chainloop/labs/wasm-policy-sdk/go => /path/to/chainloop/labs/wasm-policy-sdk/go
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
    - kind: STRING  # or SBOM_CYCLONEDX_JSON, ATTESTATION, etc.
      path: policy.wasm
```

## TinyGo Compatibility

This SDK works within TinyGo's limitations:

**Supported**:
- Flat structs with simple types
- Slices and maps with string keys
- json.Unmarshal for parsing

**Unsupported**:
- Generics (limited support)
- Complex nested types with interfaces
- Maps with any values

**Recommended**:
```go
// Good: Simple struct
type Component struct {
    Name    string `json:"name"`
    Version string `json:"version"`
    Hashes  []Hash `json:"hashes"`
}

// Avoid: Complex types
type Complex struct {
    Metadata any              `json:"metadata"`
    Data     map[string]any   `json:"data"`
}
```

## Build Configuration

### Required Flags

```bash
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
```

- `-target=wasi` - WebAssembly System Interface
- `-buildmode=c-shared` - Minimal scheduler
- `-o policy.wasm` - Output file

### Typical File Sizes

- Simple policy: ~770KB
- SBOM policy: ~770KB
- Attestation policy: ~777KB
- HTTP policy: ~793KB

## Testing

Test policies locally before deployment:

### Quick Test

```bash
# Build policy
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go

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

# Test with debug
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
| Empty/missing fields | Fail | Field-specific |
| Wrong format | Skip | Not supported |
| API unavailable | Skip | API error |

### Automated Testing

Create a `test.sh` script:

```bash
#!/bin/bash

# Build policy
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go

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
test_policy "Invalid input" "test-data/invalid.json" "version is required"
test_policy "Wrong format" "test-data/wrong.json" "skipped\": true"
```

Make executable:
```bash
chmod +x test.sh
./test.sh
```

## Troubleshooting

### Build Errors

**Error: wasm error: unreachable**
- Cause: TinyGo reflection panic or unsupported operations
- Fix: Use simpler types, avoid reflection

**Error: undefined reference**
- Cause: Missing import or SDK version mismatch
- Fix: Check SDK import path

### Runtime Errors

**HTTP request not allowed**
- Cause: Hostname not in allowed list
- Fix: Add `--allowed-hostnames` flag

**Wrong material kind**
- Cause: policy.yaml kind doesn't match data type
- Fix: Use correct kind (SBOM_CYCLONEDX_JSON, STRING, etc.)

## Best Practices

1. Keep types simple - Flat structs work best
2. Validate early - Check input before complex logic
3. Return specific violations - Clear messages help users
4. Use skip for non-applicable cases
5. Test with real data - Use actual SBOMs/attestations
6. Log validation progress - Use logging for debugging
7. Handle errors gracefully - Always check returns

## License

Apache License 2.0 - See LICENSE file for details.
