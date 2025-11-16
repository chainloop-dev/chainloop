---
name: custom-builtin-functions
description: Create a custom builtin function to be used in the Rego policy engine
---

### Policy Engine Extension

The OPA/Rego policy engine supports custom built-in functions written in Go.

**Adding Custom Built-ins**:

1. **Create Built-in Implementation** (e.g., `pkg/policies/engine/rego/builtins/myfeature.go`):
```go
package builtins

import (
    "github.com/open-policy-agent/opa/ast"
    "github.com/open-policy-agent/opa/topdown"
    "github.com/open-policy-agent/opa/types"
)

const myFuncName = "chainloop.my_function"

func RegisterMyBuiltins() error {
    return Register(&ast.Builtin{
        Name: myFuncName,
        Description: "Description of what this function does",
        Decl: types.NewFunction(
            types.Args(types.Named("input", types.S).Description("this is the input")),
            types.Named("result", types.S).Description("this is the result"),
        ),
    }, myFunctionImpl)
}

func myFunctionImpl(bctx topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
    // Extract arguments
    input, ok := operands[0].Value.(ast.String)
    if !ok {
        return fmt.Errorf("input must be a string")
    }

    // Implement logic
    result := processInput(string(input))

    // Return result
    return iter(ast.StringTerm(result))
}

// Autoregisters on package load
func init() {
    if err := RegisterMyBuiltins(); err != nil {
        panic(fmt.Sprintf("failed to register built-ins: %v", err))
    }
}
```

2. **Use in Policies** (`*.rego`):
```rego
package example
import rego.v1

result := {
    "violations": violations,
    "skipped": false
}

violations contains msg if {
    output := chainloop.my_function(input.value)
    output != "expected"
    msg := "Function returned unexpected value"
}
```

**Guidelines**:
- Use `chainloop.*` namespace for all custom built-ins
- Functions that call third party services should be marked as non-restrictive by adding the `NonRestrictiveBuiltin` category to the builtin definition
- Always implement proper error handling and return meaningful error messages
- Use context from `BuiltinContext` for timeout/cancellation support
- Document function signatures and behavior in the `Description` field and parameter definitions
