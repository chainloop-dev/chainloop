//
// Copyright 2024-2026 The Chainloop Authors.
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

package biz

import (
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/unmarshal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentifyAndValidateRawContract(t *testing.T) {
	testData := []struct {
		filename          string
		wantFormat        unmarshal.RawFormat
		wantValidationErr bool
		wantFormatErr     bool
	}{
		{
			filename:   "contract.cue",
			wantFormat: unmarshal.RawFormatCUE,
		},
		{
			filename:   "contract.json",
			wantFormat: unmarshal.RawFormatJSON,
		},
		{
			filename:          "invalid_contract.json",
			wantValidationErr: true,
		},
		{
			filename:   "contract.yaml",
			wantFormat: unmarshal.RawFormatYAML,
		},
		{
			filename:          "invalid_contract.yaml",
			wantValidationErr: true,
		},
		{
			filename:      "invalid_format.json",
			wantFormatErr: true,
		},
		{
			filename:   "contract_v2.yaml",
			wantFormat: unmarshal.RawFormatYAML,
		},
		{
			filename:   "contract_v2.json",
			wantFormat: unmarshal.RawFormatJSON,
		},
		{
			filename:          "invalid_contract_v2.yaml",
			wantValidationErr: true,
		},
	}

	for _, tc := range testData {
		t.Run(tc.filename, func(t *testing.T) {
			// load file from testdata/contracts
			data, err := os.ReadFile("testdata/contracts/" + tc.filename)
			require.NoError(t, err)

			contract, err := identifyUnMarshalAndValidateRawContract(data)
			if tc.wantValidationErr {
				assert.Error(t, err)
				assert.True(t, IsErrValidation(err))
				return
			} else if tc.wantFormatErr {
				assert.Error(t, err)
				assert.False(t, IsErrValidation(err))
				return
			}

			require.NoError(t, err)

			assert.Equal(t, tc.wantFormat, contract.Format)
			assert.Equal(t, data, contract.Raw)
		})
	}
}

func TestContractRawEqual(t *testing.T) {
	// v2 contract with 2-space indented sequences, as a user would hand-write it
	v2TwoSpace := []byte(`apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: my-contract
spec:
  materials:
  - name: my-image
    type: CONTAINER_IMAGE
    optional: false
  - name: source-code
    type: ARTIFACT
    optional: true
  envAllowList:
  - NODE_ENV
`)

	// Same semantic contract, but re-indented the way yaml.v3 reflows sequences
	// (4-space indentation under the key). This mimics what the batch apply path
	// produces after round-tripping through a yaml.v3 Node.
	v2Reindented := []byte(`apiVersion: chainloop.dev/v1
kind: Contract
metadata:
    name: my-contract
spec:
    materials:
        - name: my-image
          type: CONTAINER_IMAGE
          optional: false
        - name: source-code
          type: ARTIFACT
          optional: true
    envAllowList:
        - NODE_ENV
`)

	// Same semantic contract expressed as JSON.
	v2JSON := []byte(`{
  "apiVersion": "chainloop.dev/v1",
  "kind": "Contract",
  "metadata": {"name": "my-contract"},
  "spec": {
    "materials": [
      {"name": "my-image", "type": "CONTAINER_IMAGE", "optional": false},
      {"name": "source-code", "type": "ARTIFACT", "optional": true}
    ],
    "envAllowList": ["NODE_ENV"]
  }
}`)

	// A genuine content change: a material type differs.
	v2Changed := []byte(`apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: my-contract
spec:
  materials:
  - name: my-image
    type: SBOM_CYCLONEDX_JSON
    optional: false
  - name: source-code
    type: ARTIFACT
    optional: true
  envAllowList:
  - NODE_ENV
`)

	testCases := []struct {
		name      string
		a         []byte
		b         []byte
		wantEqual bool
	}{
		{
			name:      "identical bytes",
			a:         v2TwoSpace,
			b:         v2TwoSpace,
			wantEqual: true,
		},
		{
			name:      "same contract, different YAML indentation",
			a:         v2TwoSpace,
			b:         v2Reindented,
			wantEqual: true,
		},
		{
			name:      "same contract, YAML vs JSON",
			a:         v2TwoSpace,
			b:         v2JSON,
			wantEqual: true,
		},
		{
			name:      "genuine content change is detected",
			a:         v2TwoSpace,
			b:         v2Changed,
			wantEqual: false,
		},
		{
			name:      "unparseable input falls back to raw byte comparison (equal)",
			a:         []byte("not a contract"),
			b:         []byte("not a contract"),
			wantEqual: true,
		},
		{
			name:      "unparseable input falls back to raw byte comparison (different)",
			a:         []byte("not a contract"),
			b:         []byte("also not a contract"),
			wantEqual: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantEqual, ContractRawEqual(tc.a, tc.b))
			// equality must be symmetric
			assert.Equal(t, tc.wantEqual, ContractRawEqual(tc.b, tc.a))
		})
	}
}
