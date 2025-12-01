# Simple Policy Example

A basic policy that validates string input, demonstrating fundamental SDK usage patterns.

## What This Policy Does

Validates a message string with three rules:
1. Message must not be empty
2. Message must not contain forbidden words ("forbidden", "banned", "prohibited")
3. Message must not exceed 100 characters

## Building

```bash
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
```

## Testing

### Valid input:
```bash
echo '{"message": "hello world"}' | \
  chainloop policy develop eval \
  --policy policy.yaml \
  --material - \
  --kind ATTESTATION
```

Expected: No violations

### Invalid input (empty message):
```bash
echo '{"message": ""}' | \
  chainloop policy develop eval \
  --policy policy.yaml \
  --material - \
  --kind ATTESTATION
```

Expected: Violation: "message cannot be empty"

### Invalid input (forbidden word):
```bash
echo '{"message": "this is forbidden"}' | \
  chainloop policy develop eval \
  --policy policy.yaml \
  --material - \
  --kind ATTESTATION
```

Expected: Violation: "message contains forbidden word: forbidden"

### Invalid input (too long):
```bash
echo '{"message": "'"$(python3 -c "print('a' * 101)")"'"}' | \
  chainloop policy develop eval \
  --policy policy.yaml \
  --material - \
  --kind ATTESTATION
```

Expected: Violation: "message too long: 101 characters (max 100)"

## Key Concepts Demonstrated

1. **ExecutePolicyTyped**: Type-safe policy function with automatic I/O
2. **Result builders**: `Success()`, `AddViolation()`, `AddViolationf()`
3. **Logging**: `LogInfo()`, `LogError()` for debug output
4. **Result checking**: `HasViolations()` to check result state

## File Structure

```
simple/
├── policy.go       # Policy implementation
├── policy.yaml     # Policy specification
├── policy.wasm     # Compiled WASM module (after build)
└── README.md       # This file
```
