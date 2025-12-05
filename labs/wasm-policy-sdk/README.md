# Chainloop WASM Policy SDKs

Official SDKs for writing [Chainloop](https://chainloop.dev) policies that compile to WebAssembly.

## Overview

Chainloop policies are validation rules that run automatically when artifacts (SBOMs, attestations, evidence files) are uploaded to Chainloop. These SDKs make it easy to write policies in your preferred language that compile to WASM and execute securely.

## Available SDKs

### Go SDK

Write policies in Go that compile to WASM using TinyGo.

- **Language:** Go
- **Compiler:** TinyGo
- **Documentation:** [go/README.md](./go/README.md)

```go
import chainlooppolicy "github.com/chainloop-dev/chainloop/policies/go"

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
```

### JavaScript SDK

Write policies in JavaScript or TypeScript that compile to WASM using Extism.

- **Language:** JavaScript/TypeScript
- **Compiler:** esbuild + extism-js
- **Documentation:** [js/README.md](./js/README.md)

```javascript
const { getMaterialJSON, success, outputResult, run } = require('@chainloop-dev/policy-sdk');

function Execute() {
  return run(() => {
    const input = getMaterialJSON();

    const result = success();
    if (input.message === "") {
      result.addViolation("message cannot be empty");
    }

    outputResult(result);
  });
}
```

## Quick Start

Choose your preferred language and follow the respective README:

- [Go SDK Quick Start](./go/README.md#quick-start)
- [JavaScript SDK Quick Start](./js/README.md#quick-start)

## Features

Both SDKs provide:

- **Simple API** - Minimal boilerplate, focus on validation logic
- **Material Access** - Parse and validate JSON, strings, or raw bytes
- **Policy Arguments** - Configurable validation rules
- **HTTP Support** - Make external API calls (with hostname restrictions)
- **Artifact Discovery** - Explore the artifact graph and related attestations
- **Logging** - Debug and trace policy execution
- **Result Building** - Success, failure, and skip states with violation messages
- **Examples** - Working examples for common use cases

## Examples

Each SDK includes working examples in the `examples/` directory:

- **simple** - Basic validation with arguments
- **sbom** - CycloneDX SBOM validation
- **attestation** - in-toto attestation validation
- **http** - External API integration
- **discover** - Artifact graph exploration

## License

Apache License 2.0
