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

// HTTP example demonstrating how policies can make external API calls
// with hostname restrictions enforced by the policy engine.
//
// This example shows:
// - HTTPGet() to fetch data from allowed hostnames
// - HTTPGetJSON() to parse JSON responses
// - Hostname blocking for security
//
// Build:
//
//	tinygo build -target=wasi -buildmode=c-shared -o policy.wasm policy.go
//
// Test with allowed hostname:
//
//	chainloop policy develop eval \
//	  --policy policy.yaml \
//	  --material data.json \
//	  --kind EVIDENCE \
//	  --allowed-hostnames dummyjson.com
//
// Test with blocked hostname (will fail):
//
//	chainloop policy develop eval \
//	  --policy policy.yaml \
//	  --material data.json \
//	  --kind EVIDENCE \
//	  --allowed-hostnames www.example.com
package main

import (
	chainlooppolicy "github.com/chainloop-dev/chainloop/sdks/go"
)

type Input struct {
	CheckURL string `json:"check_url"`
}

type APIResponse struct {
	Products []Product `json:"products"`
}

type Product struct {
	ID    int     `json:"id"`
	Title string  `json:"title"`
	Price float64 `json:"price"`
}

//export Execute
func Execute() int32 {
	return chainlooppolicy.Run(func() {
		// Parse material
		var input Input
		if err := chainlooppolicy.GetMaterialJSON(&input); err != nil {
			chainlooppolicy.OutputResult(chainlooppolicy.Skip(err.Error()))
			return
		}

		chainlooppolicy.LogInfo("Making HTTP request to: %s", input.CheckURL)

		// Attempt HTTP request (will be blocked if hostname not allowed)
		var apiResp APIResponse
		if err := chainlooppolicy.HTTPGetJSON(input.CheckURL, &apiResp); err != nil {
			// This will fail if hostname is not in allowed list
			chainlooppolicy.LogError("HTTP request failed: %v", err)
			chainlooppolicy.OutputResult(chainlooppolicy.Fail(
				"failed to fetch data: " + err.Error(),
			))
			return
		}

		chainlooppolicy.LogInfo("HTTP request succeeded")
		chainlooppolicy.OutputResult(chainlooppolicy.Success())
	})
}

func main() {}
