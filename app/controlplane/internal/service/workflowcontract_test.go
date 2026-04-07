//
// Copyright 2026 The Chainloop Authors.
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

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		rawContract []byte
		wantName    string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty content returns nil",
			rawContract: []byte{},
			wantName:    "",
			wantErr:     false,
		},
		{
			name: "valid v2 contract returns metadata",
			rawContract: []byte(`
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: my-contract
  description: a test contract
spec:
  materials:
    - type: ARTIFACT
      name: my-artifact
`),
			wantName: "my-contract",
			wantErr:  false,
		},
		{
			name: "v2 contract with structural error returns parsing error",
			rawContract: []byte(`
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: my-contract
spec:
  materials:
    ref: vulnerabilities
`),
			wantErr:     true,
			errContains: "invalid contract",
		},
		{
			name: "non-v2 content returns nil",
			rawContract: []byte(`
schemaVersion: v1
materials:
  - type: ARTIFACT
    name: my-artifact
`),
			wantName: "",
			wantErr:  false,
		},
		{
			name:        "unparseable content returns nil",
			rawContract: []byte("\x00\x01\x02"),
			wantName:    "",
			wantErr:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			metadata, err := extractMetadata(tc.rawContract)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}

			require.NoError(t, err)
			if tc.wantName == "" {
				assert.Nil(t, metadata)
			} else {
				require.NotNil(t, metadata)
				assert.Equal(t, tc.wantName, metadata.GetName())
			}
		})
	}
}

func TestValidateAndExtractMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		rawContract  []byte
		explicitName string
		explicitDesc string
		wantName     string
		wantDesc     *string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "explicit name with no contract",
			rawContract:  nil,
			explicitName: "my-workflow",
			wantName:     "my-workflow",
			wantErr:      false,
		},
		{
			name:        "no name and no contract returns error",
			rawContract: nil,
			wantErr:     true,
			errContains: "name is required",
		},
		{
			name: "v2 contract with structural error surfaces parsing error",
			rawContract: []byte(`
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: my-contract
spec:
  materials:
    ref: vulnerabilities
`),
			wantErr:     true,
			errContains: "invalid contract",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			name, desc, err := validateAndExtractMetadata(tc.rawContract, tc.explicitName, tc.explicitDesc)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantName, name)
			if tc.wantDesc != nil {
				require.NotNil(t, desc)
				assert.Equal(t, *tc.wantDesc, *desc)
			}
		})
	}
}
