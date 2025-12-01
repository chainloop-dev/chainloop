# Attestation Validation Policy Example

Validates Sigstore bundles to ensure attestations reference signed git commits.

## What This Policy Does

Validates a Chainloop attestation (Sigstore bundle format) with these rules:

1. Must contain a `git.head` subject (git commit reference)
2. The git commit must have a SHA-1 digest
3. The git commit must have a signature annotation

## Building

```bash
tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
```

## Testing

This policy requires actual Chainloop attestation data. You can test it using:

### With real attestation data:
```bash
# From Chainloop managed repo
chainloop policy develop eval \
  --policy policy.yaml \
  --material pkg/attestation/crafter/materials/testdata/attestation-bundle.json \
  --kind ATTESTATION \
  --debug
```

### Expected behavior:

**Valid attestation** (with signed commit):
```
Attestation validation passed
No violations
```

**Missing git commit**:
```
Violation: "attestation must reference a git commit"
```

**Commit without signature**:
```
Violation: "git commit must have a valid signature"
```

## Key Concepts Demonstrated

1. **Sigstore bundle parsing**: Working with protobuf-based Sigstore bundles
2. **DSSE envelope handling**: Using `ParseDSSEPayloadFromBundle()` helper
3. **Attestation structure**: Understanding in-toto attestation subjects
4. **Subject lookups**: Using `FindSubjectByName()` helper
5. **Annotation access**: Reading metadata from subject annotations
6. **Type assertions**: Safely accessing `map[string]any` annotations

## Understanding the Data Flow

```
Sigstore Bundle (protobuf)
    ↓
DSSEEnvelopeFromBundle()
    ↓
DSSE Envelope
    ↓
DecodeB64Payload()
    ↓
Attestation JSON
    ↓
ParseJSON()
    ↓
Attestation struct
    ↓
Access Subjects
```

The SDK handles all these conversions automatically with `ParseDSSEPayloadFromBundle()`.

## File Structure

```
attestation/
├── policy.go       # Policy implementation
├── policy.yaml     # Policy specification
├── policy.wasm     # Compiled WASM module (after build)
└── README.md       # This file
```

## Real-World Usage

This policy ensures that all code changes in your supply chain are:
1. Associated with specific git commits
2. Signed by authorized developers

```yaml
# workflow-contract.yaml
spec:
  policies:
    - ref: file://./git-commit-signature-policy.yaml
      with:
        name: signed-commits-required
```

This policy will automatically validate all attestations created by Chainloop workflows.

## Advanced: Custom Signature Validation

You can extend this policy to validate signature format or cryptographic properties:

```go
func validateSignatureFormat(signature string) bool {
    // PGP signature starts with -----BEGIN PGP SIGNATURE-----
    return strings.HasPrefix(signature, "-----BEGIN PGP SIGNATURE-----")
}

func hasCommitSignature(subjects []chainlooppolicy.Subject) bool {
    for _, sub := range subjects {
        if sub.Name == "git.head" {
            if sig, ok := sub.Annotations["signature"].(string); ok {
                return validateSignatureFormat(sig)
            }
        }
    }
    return false
}
```
