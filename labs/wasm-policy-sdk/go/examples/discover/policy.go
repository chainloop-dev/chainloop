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

// Discover policy example that checks for policy violations in related attestations.
//
// This policy validates that container images do not have any related attestations
// with policy violations. It uses the discover builtin to explore the artifact graph
// and check attestation metadata.
//
// This example demonstrates:
// - Extracting digest from container image metadata
// - Using the Discover() function to explore artifact relationships
// - Checking for policy violations in related attestations
// - Processing attestation metadata (name, project, organization)
//
// Build:
//
//	tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
//
// Test with container image:
//
//	chainloop policy develop eval \
//	  --policy policy.yaml \
//	  --material docker://nginx:latest \
//	  --kind CONTAINER_IMAGE
package main

import (
	"fmt"

	chainlooppolicy "github.com/chainloop-dev/chainloop/labs/wasm-policy-sdk/go"
)

// ContainerImage represents the container image material structure.
type ContainerImage struct {
	ChainloopMetadata ChainloopMetadata `json:"chainloop_metadata"`
}

// ChainloopMetadata contains Chainloop-specific metadata.
type ChainloopMetadata struct {
	Digest DigestInfo `json:"digest"`
}

// DigestInfo contains digest information.
type DigestInfo struct {
	SHA256 string `json:"sha256"`
}

//export Execute
func Execute() int32 {
	return chainlooppolicy.Run(func() {
		// Parse container image material to get the digest
		var input ContainerImage
		if err := chainlooppolicy.GetMaterialJSON(&input); err != nil {
			chainlooppolicy.OutputResult(chainlooppolicy.Skip(err.Error()))
			return
		}

		if input.ChainloopMetadata.Digest.SHA256 == "" {
			chainlooppolicy.OutputResult(chainlooppolicy.Skip("no digest found in chainloop_metadata"))
			return
		}

		// Construct full digest with sha256 prefix
		digest := fmt.Sprintf("sha256:%s", input.ChainloopMetadata.Digest.SHA256)
		chainlooppolicy.LogInfo("Discovering artifacts related to: %s", digest)

		// Call the discover function to explore the artifact graph
		discoverResult, err := chainlooppolicy.Discover(digest, "")
		if err != nil {
			chainlooppolicy.LogError("Discovery failed: %v", err)
			chainlooppolicy.OutputResult(chainlooppolicy.Skip(err.Error()))
			return
		}

		if discoverResult == nil {
			chainlooppolicy.LogWarn("No discover result returned (gRPC connection may not be configured)")
			chainlooppolicy.OutputResult(chainlooppolicy.Skip("discover not available"))
			return
		}

		// Check for policy violations in related attestations
		result := checkAttestationViolations(discoverResult)

		// Output result
		chainlooppolicy.OutputResult(result)
	})
}

// checkAttestationViolations checks if any related attestations have policy violations.
func checkAttestationViolations(dr *chainlooppolicy.DiscoverResult) chainlooppolicy.Result {
	result := chainlooppolicy.Success()

	chainlooppolicy.LogInfo("Found %d references for artifact %s",
		len(dr.References), dr.Digest)

	// Check each reference for attestations with policy violations
	for _, ref := range dr.References {
		// Only check attestations
		if ref.Kind != "ATTESTATION" {
			continue
		}

		chainlooppolicy.LogInfo("Checking attestation: %s", ref.Digest)

		// Check if this attestation has policy violations
		if hasPolicyViolations, exists := ref.Metadata["hasPolicyViolations"]; exists && hasPolicyViolations == "true" {
			// Extract metadata for detailed violation message
			name := ref.Metadata["name"]
			project := ref.Metadata["project"]
			organization := ref.Metadata["organization"]

			msg := fmt.Sprintf(
				"attestation with digest %s contains policy violations [name: %s, project: %s, org: %s]",
				ref.Digest, name, project, organization,
			)
			result.AddViolation(msg)
			chainlooppolicy.LogError(msg)
		}
	}

	// Log summary
	if result.HasViolations() {
		chainlooppolicy.LogError("Validation failed: found attestations with policy violations")
	} else {
		chainlooppolicy.LogInfo("Validation passed: no related attestations have policy violations")
	}

	return result
}

func main() {}
