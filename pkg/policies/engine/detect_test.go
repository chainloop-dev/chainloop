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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectPolicyType(t *testing.T) {
	tests := []struct {
		name     string
		source   []byte
		expected PolicyType
	}{
		{
			name:     "WASM magic bytes",
			source:   []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00},
			expected: PolicyTypeWASM,
		},
		{
			name:     "Rego policy",
			source:   []byte("package main\n\nresult = {\"violations\": []}"),
			expected: PolicyTypeRego,
		},
		{
			name:     "Empty file",
			source:   []byte{},
			expected: PolicyTypeRego,
		},
		{
			name:     "Partial WASM magic bytes (only 3 bytes)",
			source:   []byte{0x00, 0x61, 0x73},
			expected: PolicyTypeRego,
		},
		{
			name:     "Incorrect magic bytes",
			source:   []byte{0x00, 0x61, 0x73, 0x6e},
			expected: PolicyTypeRego,
		},
		{
			name:     "Binary data that's not WASM",
			source:   []byte{0xFF, 0xD8, 0xFF, 0xE0}, // JPEG magic bytes
			expected: PolicyTypeRego,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectPolicyType(tt.source)
			assert.Equal(t, tt.expected, result)
		})
	}
}
