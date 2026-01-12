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

// Simple policy example that validates basic string input.
//
// This example demonstrates:
// - Using Run() wrapper to hide WASM return values
// - Material extraction using GetMaterialJSON()
// - Args extraction for configuration
// - Result building with helper methods
// - Logging with LogInfo()
//
// Build:
//
//	tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
//
// Test:
//
//	echo '{"message": "hello"}' > /tmp/test-message.json
//	chainloop policy develop eval \
//	  --policy policy.yaml \
//	  --material /tmp/test-message.json \
//	  --kind EVIDENCE
package main

import (
	"fmt"
	"strings"

	chainlooppolicy "github.com/chainloop-dev/chainloop/labs/wasm-policy-sdk/go"
)

// Input represents the expected input structure.
type Input struct {
	Message string `json:"message"`
}

//export Execute
func Execute() int32 {
	return chainlooppolicy.Run(func() {
		// Extract policy arguments (optional configuration)
		args, err := chainlooppolicy.GetArgs()
		if err != nil {
			chainlooppolicy.OutputResult(chainlooppolicy.Skip(err.Error()))
			return
		}

		// Get max length from args, default to 100
		maxLength := 100
		if maxLengthStr, ok := args["max_length"]; ok {
			fmt.Sscanf(maxLengthStr, "%d", &maxLength)
		}

		chainlooppolicy.LogInfo("Validating message with max length: %d", maxLength)

		// Parse material
		var input Input
		if err := chainlooppolicy.GetMaterialJSON(&input); err != nil {
			chainlooppolicy.OutputResult(chainlooppolicy.Skip(err.Error()))
			return
		}

		// Validate
		result := validateMessage(input, maxLength)

		// Output result
		chainlooppolicy.OutputResult(result)
	})
}

// validateMessage checks the message for compliance.
func validateMessage(input Input, maxLength int) chainlooppolicy.Result {
	result := chainlooppolicy.Success()

	// Validation 1: Message must not be empty
	if input.Message == "" {
		result.AddViolation("message cannot be empty")
		return result
	}

	// Validation 2: Message must not contain forbidden words
	forbidden := []string{"forbidden", "banned", "prohibited"}
	for _, word := range forbidden {
		if strings.Contains(strings.ToLower(input.Message), word) {
			result.AddViolationf("message contains forbidden word: %s", word)
		}
	}

	// Validation 3: Message must not be too long
	if len(input.Message) > maxLength {
		result.AddViolationf("message too long: %d characters (max %d)", len(input.Message), maxLength)
	}

	if result.HasViolations() {
		chainlooppolicy.LogError("Validation failed with %d violations", len(result.Violations))
	} else {
		chainlooppolicy.LogInfo("Validation passed for message: %s", input.Message)
	}

	return result
}

func main() {}
