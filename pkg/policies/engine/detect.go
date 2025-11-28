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

package engine

// PolicyType represents the type of a policy (Rego or WASM)
type PolicyType string

const (
	// PolicyTypeRego indicates a Rego-based policy
	PolicyTypeRego PolicyType = "rego"
	// PolicyTypeWASM indicates a WASM-based policy
	PolicyTypeWASM PolicyType = "wasm"
)

// DetectPolicyType determines the policy type from source bytes
// WASM files start with magic bytes: 0x00 0x61 0x73 0x6d (\0asm)
// as documented at https://webassembly.github.io/spec/core/binary/modules.html#binary-module
func DetectPolicyType(source []byte) PolicyType {
	// WASM files start with magic bytes: 0x00 0x61 0x73 0x6d (\0asm)
	if len(source) >= 4 &&
		source[0] == 0x00 &&
		source[1] == 0x61 &&
		source[2] == 0x73 &&
		source[3] == 0x6d {
		return PolicyTypeWASM
	}

	// Default to Rego (text-based)
	return PolicyTypeRego
}
