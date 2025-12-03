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

// Attestation policy that validates in-toto attestations for git commits.
//
// This example demonstrates:
// - Using Run() wrapper for clean API
// - Parsing in-toto attestation materials
// - Complex validation logic with nested structs
// - Logging validation details
//
// Build:
//
//	tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
//
// Test:
//
//	chainloop policy develop eval \
//	  --policy policy.yaml \
//	  --material attestation.json \
//	  --kind ATTESTATION
package main

import (
	"strings"

	chainlooppolicy "github.com/chainloop-dev/chainloop/sdks/go"
)

// Attestation represents an in-toto attestation.
type Attestation struct {
	Type          string    `json:"_type"`
	Subject       []Subject `json:"subject"`
	PredicateType string    `json:"predicateType"`
}

// Subject represents an attestation subject.
type Subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

//export Execute
func Execute() int32 {
	return chainlooppolicy.Run(func() {
		// Parse material
		var attestation Attestation
		if err := chainlooppolicy.GetMaterialJSON(&attestation); err != nil {
			chainlooppolicy.OutputResult(chainlooppolicy.Skip(err.Error()))
			return
		}

		chainlooppolicy.LogInfo("Validating in-toto attestation with %d subjects", len(attestation.Subject))

		// Skip if not in-toto attestation
		if attestation.Type != "https://in-toto.io/Statement/v0.1" &&
			attestation.Type != "https://in-toto.io/Statement/v1" {
			chainlooppolicy.OutputResult(chainlooppolicy.Skip("not an in-toto attestation"))
			return
		}

		// Validate
		result := validateAttestation(attestation)

		if result.HasViolations() {
			chainlooppolicy.LogError("Attestation validation failed with %d violations", len(result.Violations))
		} else {
			chainlooppolicy.LogInfo("Attestation validation passed")
		}

		// Output result
		chainlooppolicy.OutputResult(result)
	})
}

// validateAttestation checks the attestation for compliance.
func validateAttestation(attestation Attestation) chainlooppolicy.Result {
	result := chainlooppolicy.Success()

	// Check subjects exist
	if len(attestation.Subject) == 0 {
		result.AddViolation("attestation must contain at least one subject")
		return result
	}

	// Check for git commit subject
	hasGitCommit := false
	for _, subject := range attestation.Subject {
		if subject.Name == "git.head" {
			hasGitCommit = true
			chainlooppolicy.LogDebug("Found git.head subject")

			// Check if git commit has SHA1 digest
			sha1, hasSha1 := subject.Digest["sha1"]
			if !hasSha1 {
				result.AddViolation("git.head subject missing sha1 digest")
			} else if sha1 == "" {
				result.AddViolation("git.head subject has empty sha1 digest")
			} else if len(sha1) != 40 {
				result.AddViolationf("git.head sha1 digest has invalid length: %d (expected 40)", len(sha1))
			} else if !isValidHex(sha1) {
				result.AddViolation("git.head sha1 digest contains invalid characters")
			} else {
				chainlooppolicy.LogDebug("Valid git commit SHA1: %s", sha1)
			}
		}
	}

	if !hasGitCommit {
		result.AddViolation("attestation must reference a git commit (git.head)")
	}

	// Check predicate type is not empty
	if attestation.PredicateType == "" {
		result.AddViolation("attestation must have a predicateType")
	}

	return result
}

// isValidHex checks if a string contains only hexadecimal characters.
func isValidHex(s string) bool {
	for _, c := range strings.ToLower(s) {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func main() {}
