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

// Package chainlooppolicy provides a high-level SDK for writing Chainloop WASM policies in Go.
//
// This package builds on top of the Extism Go PDK to provide ergonomic APIs for policy
// development with clean separation of material data and policy arguments.
//
// # Quick Start
//
// A typical policy follows this pattern:
//
//	package main
//
//	import chainlooppolicy "github.com/chainloop-dev/chainloop/policies/go"
//
//	type SBOM struct {
//	    Components []Component `json:"components"`
//	}
//
//	//export Execute
//	func Execute() int32 {
//	    return chainlooppolicy.Run(func() {
//	        // Parse material
//	        var sbom SBOM
//	        if err := chainlooppolicy.GetMaterialJSON(&sbom); err != nil {
//	            chainlooppolicy.OutputResult(chainlooppolicy.Skip(err.Error()))
//	            return
//	        }
//
//	        // Validate
//	        result := chainlooppolicy.Success()
//	        if len(sbom.Components) == 0 {
//	            result.AddViolation("SBOM must have components")
//	        }
//
//	        // Output
//	        chainlooppolicy.OutputResult(result)
//	    })
//	}
//
//	func main() {}
//
// # Key Features
//
//   - Material Extraction: GetMaterialJSON(), GetMaterialBytes()
//   - Args Extraction: GetArgs(), GetArgString()
//   - Result Building: Success(), Fail(), Skip()
//   - Logging: LogInfo(), LogDebug(), LogWarn(), LogError()
//   - HTTP Requests: HTTPGet(), HTTPGetJSON(), HTTPPost(), HTTPPostJSON()
//   - Artifact Discovery: Discover(), DiscoverByDigest()
//   - Clean API: Run() wrapper hides WASM return values
//
// # Policy Arguments
//
// Policy arguments are passed via Extism config and extracted with GetArgs():
//
//	args, _ := chainlooppolicy.GetArgs()
//	threshold := args["severity_threshold"]  // Access specific arg
//
//	// Or use helpers
//	threshold := chainlooppolicy.GetArgStringDefault("severity_threshold", "HIGH")
//
// # Material Types
//
// The material is passed as the main WASM input and extracted with helpers:
//
//	// For JSON materials (SBOM, SARIF, etc.)
//	var sbom CycloneDXBOM
//	chainlooppolicy.GetMaterialJSON(&sbom)
//
//	// For raw bytes
//	data := chainlooppolicy.GetMaterialBytes()
//
//	// For text
//	text := chainlooppolicy.GetMaterialString()
//
// # Logging
//
// Logs are visible in the Chainloop CLI when running with --debug:
//
//	chainlooppolicy.LogInfo("Processing %d components", len(sbom.Components))
//	chainlooppolicy.LogDebug("Component details: %+v", component)
//	chainlooppolicy.LogWarn("Missing optional field: %s", fieldName)
//	chainlooppolicy.LogError("Failed to parse: %v", err)
//
// # HTTP Requests
//
// Policies can make HTTP requests to allowed hostnames:
//
//	// Simple GET
//	data, err := chainlooppolicy.HTTPGet("https://api.example.com/data")
//
//	// GET with JSON parsing
//	var result APIResponse
//	err := chainlooppolicy.HTTPGetJSON("https://api.example.com/data", &result)
//
//	// POST with JSON
//	err := chainlooppolicy.HTTPPostJSON("https://api.example.com/validate",
//	    requestBody, &responseBody)
//
// Note: Only hostnames configured in the policy engine's allowed list are accessible.
//
// # Artifact Discovery
//
// Policies can explore the artifact graph to discover related artifacts:
//
//	// Discover all references for an artifact
//	result, err := chainlooppolicy.Discover(digest, "")
//	if err != nil {
//	    chainlooppolicy.LogError("Discovery failed: %v", err)
//	    return
//	}
//
//	// Check references
//	for _, ref := range result.References {
//	    if ref.Kind == "ATTESTATION" {
//	        if ref.Metadata["hasPolicyViolations"] == "true" {
//	            result.AddViolation("Referenced attestation has violations")
//	        }
//	    }
//	}
//
//	// Convenience function for discovering by digest only
//	result, err := chainlooppolicy.DiscoverByDigest(digest)
//
// Note: Discovery requires a gRPC connection configured in the policy engine.
//
// # Building Policies
//
// Policies are compiled to WASM using TinyGo:
//
//	tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
//
// See the examples/ directory for complete working examples.
package _go
