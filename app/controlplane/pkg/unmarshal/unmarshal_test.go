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

package unmarshal

import (
	"os"
	"testing"

	schemav1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromRawGroupAndUnknownFields(t *testing.T) {
	// v2 contract with a choke group, expressed in the three supported formats
	yamlContract := []byte(`apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: test-contract
spec:
  materials:
    - name: a
      type: ARTIFACT
      group: choice
    - name: b
      type: ARTIFACT
      group: choice
`)
	jsonContract := []byte(`{
  "apiVersion": "chainloop.dev/v1",
  "kind": "Contract",
  "metadata": {"name": "test-contract"},
  "spec": {"materials": [
    {"name": "a", "type": "ARTIFACT", "group": "choice"},
    {"name": "b", "type": "ARTIFACT", "group": "choice"}
  ]}
}`)
	cueContract := []byte(`apiVersion: "chainloop.dev/v1"
kind: "Contract"
metadata: name: "test-contract"
spec: materials: [
	{name: "a", type: "ARTIFACT", group: "choice"},
	{name: "b", type: "ARTIFACT", group: "choice"},
]
`)

	formats := []struct {
		name   string
		format RawFormat
		body   []byte
	}{
		{"yaml", RawFormatYAML, yamlContract},
		{"json", RawFormatJSON, jsonContract},
		{"cue", RawFormatCUE, cueContract},
	}

	t.Run("group round-trips", func(t *testing.T) {
		for _, f := range formats {
			t.Run(f.name, func(t *testing.T) {
				out := &schemav1.CraftingSchemaV2{}
				require.NoError(t, FromRaw(f.body, f.format, out, true))
				materials := out.GetSpec().GetMaterials()
				require.Len(t, materials, 2)
				assert.Equal(t, "choice", materials[0].GetGroup())
				assert.Equal(t, "choice", materials[1].GetGroup())
			})
		}
	})

	// An unknown field (e.g. one added by a newer CLI) must not break parsing.
	t.Run("unknown fields are discarded", func(t *testing.T) {
		yamlUnknown := []byte(`apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: test-contract
spec:
  materials:
    - name: a
      type: ARTIFACT
      somethingFromTheFuture: true
`)
		jsonUnknown := []byte(`{
  "apiVersion": "chainloop.dev/v1",
  "kind": "Contract",
  "metadata": {"name": "test-contract"},
  "spec": {"materials": [{"name": "a", "type": "ARTIFACT", "somethingFromTheFuture": true}]}
}`)

		for _, f := range []struct {
			name   string
			format RawFormat
			body   []byte
		}{
			{"yaml", RawFormatYAML, yamlUnknown},
			{"json", RawFormatJSON, jsonUnknown},
		} {
			t.Run(f.name, func(t *testing.T) {
				out := &schemav1.CraftingSchemaV2{}
				require.NoError(t, FromRaw(f.body, f.format, out, true))
				require.Len(t, out.GetSpec().GetMaterials(), 1)
				assert.Equal(t, "a", out.GetSpec().GetMaterials()[0].GetName())
			})
		}
	})
}

func TestIdentifyFormat(t *testing.T) {
	testData := []struct {
		filename   string
		wantFormat RawFormat
		wantErr    bool
	}{
		{
			filename:   "contract.cue",
			wantFormat: RawFormatCUE,
		},
		{
			filename:   "contract.json",
			wantFormat: RawFormatJSON,
		},
		{
			filename:   "invalid_contract.json",
			wantFormat: RawFormatJSON,
		},
		{
			filename:   "contract.yaml",
			wantFormat: RawFormatYAML,
		},
		{
			filename:   "invalid_contract.yaml",
			wantFormat: RawFormatYAML,
		},
		{
			filename: "invalid_format.json",
			wantErr:  true,
		},
	}

	for _, tt := range testData {
		t.Run(tt.filename, func(t *testing.T) {
			// load file from testdata/contracts
			data, err := os.ReadFile("testdata/contracts/" + tt.filename)
			require.NoError(t, err)

			format, err := IdentifyFormat(data)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tt.wantFormat, format)
		})
	}
}
