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

package _go

import (
	"encoding/json"
	"fmt"

	"github.com/extism/go-pdk"
)

// Declare the host function provided by Chainloop
// The host function signature: (digestOffset: i64, kindOffset: i64) -> i64
//
//go:wasmimport env chainloop_discover
func chainloop_discover(digestOffset, kindOffset uint64) uint64

// DiscoverResult represents the result of a discover operation.
// It contains information about the discovered artifact and its references.
type DiscoverResult struct {
	// Digest is the artifact digest that was discovered
	Digest string `json:"digest"`
	// Kind is the material type of the discovered artifact
	Kind string `json:"kind"`
	// References contains the list of artifacts that reference or are referenced by this artifact
	References []DiscoverReference `json:"references"`
}

// DiscoverReference represents a reference to another artifact in the artifact graph.
type DiscoverReference struct {
	// Digest is the digest of the referenced artifact
	Digest string `json:"digest"`
	// Kind is the material type of the referenced artifact
	Kind string `json:"kind"`
	// Metadata contains additional information about the referenced artifact
	Metadata map[string]string `json:"metadata"`
}

// Discover calls the Chainloop discover builtin to explore the artifact graph.
// It retrieves information about artifacts related to the given digest.
//
// Parameters:
//   - digest: The artifact digest to discover (e.g., "sha256:abc123...")
//   - kind: Optional filter by material kind (e.g., "CONTAINER_IMAGE", "ATTESTATION"). Use empty string for no filter.
//
// Returns:
//   - *DiscoverResult: Information about the discovered artifact and its references
//   - error: Error if the discovery fails
//
// Example:
//
//	// Discover all references for a container image
//	digest := "sha256:abc123..."
//	result, err := chainlooppolicy.Discover(digest, "")
//	if err != nil {
//	    chainlooppolicy.LogError("Discovery failed: %v", err)
//	    return
//	}
//
//	// Check if any referenced attestations have policy violations
//	for _, ref := range result.References {
//	    if ref.Kind == "ATTESTATION" {
//	        if ref.Metadata["hasPolicyViolations"] == "true" {
//	            chainlooppolicy.LogWarn("Attestation %s has policy violations", ref.Digest)
//	        }
//	    }
//	}
//
// Note: The discover functionality requires a gRPC connection to be configured
// in the policy engine. Only artifacts accessible to your organization will be returned.
func Discover(digest, kind string) (*DiscoverResult, error) {
	// Allocate memory for digest string
	digestMem := pdk.AllocateString(digest)

	// Allocate memory for kind string (empty string if not provided)
	kindMem := pdk.AllocateString(kind)

	// Call the host function directly
	resultOffset := chainloop_discover(digestMem.Offset(), kindMem.Offset())

	// Check if result is zero (error from host function)
	if resultOffset == 0 {
		return nil, fmt.Errorf("discover returned error (check if gRPC connection is configured)")
	}

	// Read the result JSON from memory
	resultBytes := pdk.FindMemory(resultOffset)
	if resultBytes.ReadBytes() == nil {
		return nil, fmt.Errorf("failed to read discover result from memory")
	}

	// Parse the JSON result
	var result DiscoverResult
	if err := json.Unmarshal(resultBytes.ReadBytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse discover result: %w", err)
	}

	return &result, nil
}

// DiscoverByDigest is a convenience function that calls Discover with no kind filter.
// It retrieves all artifacts related to the given digest regardless of their type.
//
// Parameters:
//   - digest: The artifact digest to discover
//
// Returns:
//   - *DiscoverResult: Information about the discovered artifact and its references
//   - error: Error if the discovery fails
//
// Example:
//
//	result, err := chainlooppolicy.DiscoverByDigest("sha256:abc123...")
func DiscoverByDigest(digest string) (*DiscoverResult, error) {
	return Discover(digest, "")
}
