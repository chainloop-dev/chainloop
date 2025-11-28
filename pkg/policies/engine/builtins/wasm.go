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

package builtins

import (
	"context"
	"encoding/json"

	extism "github.com/extism/go-sdk"
	"google.golang.org/grpc"
)

// CreateDiscoverHostFunction creates an Extism host function for the discover builtin
// This allows WASM policies to call chainloop_discover(digest, kind) and get artifact graph data
func CreateDiscoverHostFunction(conn *grpc.ClientConn) extism.HostFunction {
	discoverSvc := NewDiscoverService(conn)

	return extism.NewHostFunctionWithStack(
		"chainloop_discover",
		func(ctx context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
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
			if err != nil {
				// Return 0 to signal error
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

			// Write JSON to WASM memory and return offset
			offset, err := plugin.WriteBytes(jsonData)
			if err != nil {
				// Return 0 to signal error
				stack[0] = 0
				return
			}

			stack[0] = offset
		},
		// inputs: digest offset, kind offset
		[]extism.ValueType{extism.ValueTypeI64, extism.ValueTypeI64},
		// output: json result offset or 0 on error
		[]extism.ValueType{extism.ValueTypeI64},
	)
}
