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

package action

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAndExtractName(t *testing.T) {
	contractWithName := `apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: container-image-contract
spec:
  materials:
    - type: CONTAINER_IMAGE
      name: image
`

	contractWithoutName := `schemaVersion: v1
materials:
  - type: CONTAINER_IMAGE
    name: image
`

	tests := []struct {
		name         string
		explicitName string
		fileContent  string
		wantName     string
		wantErr      string
	}{
		{
			name:         "explicit name only, no file",
			explicitName: "my-contract",
			wantName:     "my-contract",
		},
		{
			name:        "metadata.name only",
			fileContent: contractWithName,
			wantName:    "container-image-contract",
		},
		{
			name:         "explicit name matching metadata.name is accepted",
			explicitName: "container-image-contract",
			fileContent:  contractWithName,
			wantName:     "container-image-contract",
		},
		{
			name:         "explicit name differing from metadata.name errors with actionable message",
			explicitName: "another-name",
			fileContent:  contractWithName,
			wantErr:      `--name "another-name" and metadata.name "container-image-contract" differ: pass only one, or set them to the same value`,
		},
		{
			name:    "neither name nor file",
			wantErr: "name is required when no file is provided",
		},
		{
			name:        "file without metadata.name and no explicit name",
			fileContent: contractWithoutName,
			wantErr:     "name is required: either provide explicit name or include metadata.name in the schema",
		},
		{
			name:         "explicit name with file without metadata.name",
			explicitName: "my-contract",
			fileContent:  contractWithoutName,
			wantName:     "my-contract",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var filePath string
			if tc.fileContent != "" {
				filePath = filepath.Join(t.TempDir(), "contract.yaml")
				require.NoError(t, os.WriteFile(filePath, []byte(tc.fileContent), 0o600))
			}

			got, err := ValidateAndExtractName(tc.explicitName, filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantName, got)
		})
	}
}
