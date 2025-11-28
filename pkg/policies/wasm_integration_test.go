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

package policies

import (
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/stretchr/testify/assert"
)

// TestWASMPolicyTypeDetection verifies that WASM policies are detected correctly
func TestWASMPolicyTypeDetection(t *testing.T) {
	tests := []struct {
		name     string
		source   []byte
		expected engine.PolicyType
	}{
		{
			name:     "Rego policy source",
			source:   []byte("package chainloop\n\nresult = {\"violations\": []}"),
			expected: engine.PolicyTypeRego,
		},
		{
			name:     "WASM policy source",
			source:   []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00},
			expected: engine.PolicyTypeWASM,
		},
		{
			name:     "Empty source defaults to Rego",
			source:   []byte{},
			expected: engine.PolicyTypeRego,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detected := engine.DetectPolicyType(tt.source)
			assert.Equal(t, tt.expected, detected)
		})
	}
}
