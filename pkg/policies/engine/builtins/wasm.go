//
// Copyright 2024-2025 The Chainloop Authors.
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

package builtins

import (
	"context"
	"encoding/json"

	extism "github.com/extism/go-sdk"
	"google.golang.org/grpc"
)

// CreateDiscoverHostFunctions creates Extism host functions for the discover builtin.
// Returns two host functions - one for each supported namespace:
// 1. "env" namespace for Go (TinyGo) policies
// 2. "extism:host/user" namespace for JavaScript policies
func CreateDiscoverHostFunctions(conn *grpc.ClientConn) []extism.HostFunction {
	discoverSvc := NewDiscoverService(conn)

	// Shared implementation for the host function
	impl := func(ctx context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
		// Read digest from WASM memory
		digestOffset := stack[0]
		digest, err := plugin.ReadString(digestOffset)
		if err != nil {
			// Return 0 to signal error
			stack[0] = 0
			return
		}

		// Read kind from WASM memory (if provided)
		var kind string
		if len(stack) > 1 && stack[1] != 0 {
			kindOffset := stack[1]
			kind, _ = plugin.ReadString(kindOffset)
		}

		// Call shared discover service
		resp, err := discoverSvc.Discover(ctx, digest, kind)
		if err != nil || resp == nil {
			// Return 0 to signal error (no connection or error)
			stack[0] = 0
			return
		}

		// Serialize response to JSON
		jsonData, err := json.Marshal(resp.Result)
		if err != nil {
			// Return 0 to signal error
			stack[0] = 0
			return
		}

		// Write JSON string to WASM memory and return offset
		offset, err := plugin.WriteString(string(jsonData))
		if err != nil {
			// Return 0 to signal error
			stack[0] = 0
			return
		}

		stack[0] = offset
	}

	// inputs: digest offset, kind offset
	inputs := []extism.ValueType{extism.ValueTypeI64, extism.ValueTypeI64}
	// output: json result offset or 0 on error
	outputs := []extism.ValueType{extism.ValueTypeI64}

	// Create host function for "env" namespace (Go/TinyGo policies)
	envFunc := extism.NewHostFunctionWithStack("chainloop_discover", impl, inputs, outputs)
	envFunc.SetNamespace("env")

	// Create host function for "extism:host/user" namespace (JavaScript policies)
	jsFunc := extism.NewHostFunctionWithStack("chainloop_discover", impl, inputs, outputs)
	jsFunc.SetNamespace("extism:host/user")

	return []extism.HostFunction{envFunc, jsFunc}
}
