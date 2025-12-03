//
// Copyright 2025 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SBOM policy example that validates CycloneDX BOMs.
//
// This example demonstrates:
// - Using Run() wrapper for clean API
// - Parsing CycloneDX SBOM materials
// - Logging validation progress
// - Result building with violations
//
// Build:
//
//	tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
//
// Test:
//
//	chainloop policy develop eval \
//	  --policy policy.yaml \
//	  --material sbom.json \
//	  --kind SBOM_CYCLONEDX_JSON
package main

import (
	chainlooppolicy "github.com/chainloop-dev/chainloop/policies/go"
)

// CycloneDXBOM represents a CycloneDX SBOM structure.
type CycloneDXBOM struct {
	BOMFormat  string      `json:"bomFormat"`
	Components []Component `json:"components"`
}

// Component represents a software component in the SBOM.
type Component struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

//export Execute
func Execute() int32 {
	return chainlooppolicy.Run(func() {
		// Parse material
		var sbom CycloneDXBOM
		if err := chainlooppolicy.GetMaterialJSON(&sbom); err != nil {
			chainlooppolicy.OutputResult(chainlooppolicy.Skip(err.Error()))
			return
		}

		chainlooppolicy.LogInfo("Validating CycloneDX SBOM with %d components", len(sbom.Components))

		// Skip if not CycloneDX
		if sbom.BOMFormat != "CycloneDX" {
			chainlooppolicy.OutputResult(chainlooppolicy.Skip("not a CycloneDX SBOM"))
			return
		}

		// Validate
		result := validateSBOM(sbom)

		if result.HasViolations() {
			chainlooppolicy.LogError("SBOM validation failed with %d violations", len(result.Violations))
		} else {
			chainlooppolicy.LogInfo("SBOM validation passed")
		}

		// Output result
		chainlooppolicy.OutputResult(result)
	})
}

// validateSBOM checks the SBOM for compliance.
func validateSBOM(sbom CycloneDXBOM) chainlooppolicy.Result {
	result := chainlooppolicy.Success()

	// Check components exist
	if len(sbom.Components) == 0 {
		result.AddViolation("SBOM must contain at least one component")
		return result
	}

	// Validate each component
	for i, comp := range sbom.Components {
		if comp.Name == "" {
			result.AddViolationf("component at index %d missing name", i)
		}
		if comp.Version == "" {
			result.AddViolationf("component '%s' missing version", comp.Name)
		}
	}

	return result
}

func main() {}
