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

package materials

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/prinfo"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChainloopPRInfoCrafter_Validation(t *testing.T) {
	testCases := []struct {
		name    string
		data    *prinfo.Evidence
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid GitHub PR",
			data: prinfo.NewEvidence(prinfo.Data{
				Platform:     "github",
				Type:         "pull_request",
				Number:       "123",
				Title:        "Test PR",
				Description:  "Test description",
				SourceBranch: "feature",
				TargetBranch: "main",
				URL:          "https://github.com/org/repo/pull/123",
				Author:       "testuser",
			}),
			wantErr: false,
		},
		{
			name: "valid GitLab MR minimal",
			data: prinfo.NewEvidence(prinfo.Data{
				Platform: "gitlab",
				Type:     "merge_request",
				Number:   "456",
				URL:      "https://gitlab.com/org/repo/-/merge_requests/456",
			}),
			wantErr: false,
		},
		{
			name: "invalid platform",
			data: prinfo.NewEvidence(prinfo.Data{
				Platform: "bitbucket", // Invalid platform
				Type:     "pull_request",
				Number:   "123",
				URL:      "https://bitbucket.org/org/repo/pull/123",
			}),
			wantErr: true,
		},
		{
			name: "missing required field: platform",
			data: &prinfo.Evidence{
				ID:     prinfo.EvidenceID,
				Schema: prinfo.EvidenceSchemaURL,
				Data: prinfo.Data{
					// Platform is missing
					Type:   "pull_request",
					Number: "123",
					URL:    "https://github.com/org/repo/pull/123",
				},
			},
			wantErr: true,
		},
		{
			name: "missing required field: url",
			data: prinfo.NewEvidence(prinfo.Data{
				Platform: "github",
				Type:     "pull_request",
				Number:   "123",
				// URL is missing
			}),
			wantErr: true,
		},
		{
			name: "invalid type value",
			data: prinfo.NewEvidence(prinfo.Data{
				Platform: "github",
				Type:     "issue", // Invalid type
				Number:   "123",
				URL:      "https://github.com/org/repo/pull/123",
			}),
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Marshal the data field to validate it
			dataBytes, err := json.Marshal(tc.data.Data)
			require.NoError(t, err)

			var rawData interface{}
			err = json.Unmarshal(dataBytes, &rawData)
			require.NoError(t, err)

			// Validate the data against JSON schema
			err = schemavalidators.ValidatePRInfo(rawData, schemavalidators.PRInfoVersion1_0)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChainloopPRInfoCrafter_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(tmpFile, []byte(`{invalid json}`), 0600)
	require.NoError(t, err)

	// Read and try to unmarshal
	f, err := os.ReadFile(tmpFile)
	require.NoError(t, err)

	var v prinfo.Evidence
	err = json.Unmarshal(f, &v)
	require.Error(t, err)
}

func TestChainloopPRInfoCrafter_WrongMaterialType(t *testing.T) {
	logger := zerolog.Nop()

	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON, // Wrong type
	}

	_, err := NewChainloopPRInfoCrafter(schema, nil, &logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "material type is not chainloop_pr_info")
}
