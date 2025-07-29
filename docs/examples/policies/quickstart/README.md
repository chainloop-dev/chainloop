# Policy Development Quickstart

This quickstart guide walks you through creating and validating a basic Chainloop policy that checks SBOM freshness (ensuring SBOMs are not older than 30 days). All steps are CLI-driven and can be run locally.

## Documentation References

- **CLI Reference**: [chainloop policy develop commands](https://docs.chainloop.dev/command-line-reference/cli-reference#chainloop-policy)
- **Policy Concepts**: [Understanding Chainloop Policies](https://docs.chainloop.dev/concepts/policies)
- **Custom Policy Guide**: [Writing Custom Policies](https://docs.chainloop.dev/guides/custom-policies)
- **Material Types**: [Available Material Types](https://docs.chainloop.dev/concepts/material-types#material-types)

## Quick Test

### Step 1: Download the Example Files

```bash
# Download the policy and sample materials
curl -O https://raw.githubusercontent.com/chainloop-dev/chainloop/refs/heads/main/docs/examples/policies/quickstart/cdx-fresh.yaml
curl -O https://raw.githubusercontent.com/chainloop-dev/chainloop/refs/heads/main/docs/examples/policies/quickstart/cdx-old.json
curl -O https://raw.githubusercontent.com/chainloop-dev/chainloop/refs/heads/main/docs/examples/policies/quickstart/cdx-fresh.json
```

### Step 2: Lint the Policy

Check your policy's structure and Rego syntax. Run it with `--format` flag to fix all formatting inconsistencies.

```bash
chainloop policy develop lint --policy cdx-fresh.yaml --format
```

**Expected output:**
```
INF policy is valid!
```

### Step 3: Evaluate the Policy

Use your SBOM CycloneDX material to test your policy logic.

```bash
# Test with old SBOM (should fail)
chainloop policy develop eval --policy cdx-fresh.yaml --material cdx-old.json --kind SBOM_CYCLONEDX_JSON

# Test with fresh SBOM (should pass)  
chainloop policy develop eval --policy cdx-fresh.yaml --material cdx-fresh.json --kind SBOM_CYCLONEDX_JSON
```

**Expected Results:**

**Old SBOM (should fail):**
```
INF - cdx-fresh: SBOM created at: 2024-06-15T10:30:00Z which is too old (freshness limit set to 30 days)
INF policy evaluation failed
```

**Fresh SBOM (should pass):**
```
INF policy evaluation passed
```

## Create Your Own Policy

### Step 1: Initialize a Policy Template

Create a new policy with the embedded format (single YAML file):

```bash
chainloop policy develop init --embedded --name my-policy --description "My custom policy description"
```

**Note**: This creates a file named `my-policy.yaml` (based on the `--name` parameter). Without `--embedded`, it creates separate `chainloop-policy.yaml` and `chainloop-policy.rego` files.

### Step 2: Write Your Policy Rules

Edit the generated YAML file and replace the placeholder code in the `embedded` section with your Rego logic. 

**Important**: Remove the `default violations := []` line to avoid conflicts with your `violations contains msg if` rules.

### Step 3: Test Your Policy

Follow steps 2-3 above with your own policy and materials.

## Policy Logic Explained

The SBOM freshness policy calculates a 30-day threshold in nanoseconds and compares it against the SBOM's `metadata.timestamp` field:

1. **Converts 30 days to nanoseconds**: `30 * 24 * 60 * 60 * 1000 * 1000 * 1000`
2. **Parses the SBOM timestamp** using `time.parse_rfc3339_ns()`
3. **Checks if current time minus (SBOM time + threshold) is positive**
4. **If positive, the SBOM is too old** and a violation is raised

## Available Material Types

For the complete list of supported material types, see the [Material Types documentation](https://docs.chainloop.dev/concepts/material-types#material-types).

Common material types for the `--kind` parameter:

- `SBOM_CYCLONEDX_JSON` - CycloneDX SBOM files
- `SBOM_SPDX_JSON` - SPDX SBOM files  
- `CONTAINER_IMAGE` - Container images
- `ATTESTATION` - Generic attestations
- `SARIF` - SARIF security scan results
- `SLSA_PROVENANCE` - SLSA provenance attestations

Run `chainloop policy develop eval --help` for the complete list.

## Common Issues

1. **Rego type conflicts**: Remove `default violations := []` when using `violations contains msg if` rules
2. **Missing material kind**: Always specify `--kind` parameter in eval command
3. **File naming**: Policy files are named based on the `--name` parameter, not always `policy.yaml`
4. **Time calculations**: Use nanoseconds for time comparisons in Rego policies

## Next Steps

Once you've mastered the basics:

1. Explore more complex examples in the [policy examples]](../) directory
2. Learn about policy inputs and annotations
3. Practice with different material types
4. Deploy policies to your Chainloop workflows