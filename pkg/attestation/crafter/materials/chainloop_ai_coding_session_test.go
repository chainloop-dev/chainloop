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

package materials

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	mUploader "github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChainloopAICodingSessionCrafter_WrongType(t *testing.T) {
	logger := zerolog.Nop()

	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
	}

	_, err := NewChainloopAICodingSessionCrafter(schema, nil, &logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "material type is not chainloop_ai_coding_session")
}

func TestNewChainloopAICodingSessionCrafter_CorrectType(t *testing.T) {
	logger := zerolog.Nop()

	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION,
	}

	crafter, err := NewChainloopAICodingSessionCrafter(schema, nil, &logger)
	require.NoError(t, err)
	assert.NotNil(t, crafter)
}

func TestChainloopAICodingSessionCrafter_Validation(t *testing.T) {
	testCases := []struct {
		name    string
		data    *aicodingsession.Data
		wantErr bool
	}{
		{
			name: "valid full session",
			data: &aicodingsession.Data{
				SchemaVersion: "v1",
				Agent:         aicodingsession.Agent{Name: "claude-code", Version: "2.1.83"},
				Session: aicodingsession.Session{
					ID:              "fa8acbe6-a176-4c2a-b51e-fd4541615eb5",
					Slug:            "stateful-wobbling-sutherland",
					StartedAt:       "2026-03-25T15:10:49.161Z",
					EndedAt:         "2026-03-25T16:59:14.988Z",
					DurationSeconds: 6505,
				},
				GitContext: &aicodingsession.GitContext{
					Repository: "git@github.com:example/repo.git",
					Branch:     "main",
				},
				Model: &aicodingsession.Model{
					Primary:  "claude-opus-4-6",
					Provider: "anthropic",
				},
				Usage: &aicodingsession.Usage{
					TotalTokens:      3052,
					EstimatedCostUSD: 0.84,
				},
			},
			wantErr: false,
		},
		{
			name: "valid minimal session",
			data: &aicodingsession.Data{
				SchemaVersion: "v1",
				Agent:         aicodingsession.Agent{Name: "cursor"},
				Session: aicodingsession.Session{
					ID:              "abc-123",
					StartedAt:       "2026-03-25T15:10:49.161Z",
					DurationSeconds: 100,
				},
			},
			wantErr: false,
		},
		{
			name:    "missing required fields",
			data:    &aicodingsession.Data{},
			wantErr: true,
		},
		{
			name: "missing session",
			data: &aicodingsession.Data{
				SchemaVersion: "v1",
				Agent:         aicodingsession.Agent{Name: "claude-code"},
			},
			wantErr: true,
		},
		{
			name: "missing agent name",
			data: &aicodingsession.Data{
				SchemaVersion: "v1",
				Agent:         aicodingsession.Agent{},
				Session: aicodingsession.Session{
					ID:              "abc",
					StartedAt:       "2026-03-25T15:10:49.161Z",
					DurationSeconds: 100,
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dataBytes, err := json.Marshal(tc.data)
			require.NoError(t, err)

			var rawData any
			require.NoError(t, json.Unmarshal(dataBytes, &rawData))

			err = schemavalidators.ValidateAICodingSession(rawData, schemavalidators.AICodingSessionVersion0_1)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChainloopAICodingSessionCrafter_InvalidJSON(t *testing.T) {
	logger := zerolog.Nop()
	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION,
	}

	crafter, err := NewChainloopAICodingSessionCrafter(schema, nil, &logger)
	require.NoError(t, err)

	tmpFile := filepath.Join(t.TempDir(), "invalid.json")
	require.NoError(t, os.WriteFile(tmpFile, []byte(`{invalid json}`), 0o600))

	_, err = crafter.Craft(context.Background(), tmpFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON format")
}

func TestChainloopAICodingSessionCrafter_InvalidSchema(t *testing.T) {
	logger := zerolog.Nop()
	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION,
	}

	crafter, err := NewChainloopAICodingSessionCrafter(schema, nil, &logger)
	require.NoError(t, err)

	// Valid envelope but data is missing required fields
	tmpFile := filepath.Join(t.TempDir(), "bad-schema.json")
	require.NoError(t, os.WriteFile(tmpFile, []byte(`{"chainloop.material.evidence.id":"CHAINLOOP_AI_CODING_SESSION","schema":"test","data":{"foo":"bar"}}`), 0o600))

	_, err = crafter.Craft(context.Background(), tmpFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI coding session validation failed")
}

func TestChainloopAICodingSessionCrafter_FileNotFound(t *testing.T) {
	logger := zerolog.Nop()
	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION,
	}

	crafter, err := NewChainloopAICodingSessionCrafter(schema, nil, &logger)
	require.NoError(t, err)

	_, err = crafter.Craft(context.Background(), "/nonexistent/file.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't open the file")
}

func TestChainloopAICodingSessionCrafter_RejectsExtraFields(t *testing.T) {
	logger := zerolog.Nop()
	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION,
	}

	crafter, err := NewChainloopAICodingSessionCrafter(schema, nil, &logger)
	require.NoError(t, err)

	payload := `{
		"chainloop.material.evidence.id": "CHAINLOOP_AI_CODING_SESSION",
		"schema": "https://schemas.chainloop.dev/aicodingsession/0.1/ai-coding-session.schema.json",
		"data": {
			"schema_version": "v1",
			"agent": {"name": "claude-code"},
			"session": {"id": "abc", "started_at": "2026-03-25T15:10:49.161Z", "duration_seconds": 100},
			"unknown_field": "should fail"
		}
	}`

	tmpFile := filepath.Join(t.TempDir(), "extra-fields.json")
	require.NoError(t, os.WriteFile(tmpFile, []byte(payload), 0o600))

	_, err = crafter.Craft(context.Background(), tmpFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI coding session validation failed")
}

func TestChainloopAICodingSessionCrafter_RealWorldEvidence(t *testing.T) {
	// Load real-world evidence that uses the new attribution, line range, and subagent fields.
	raw, err := os.ReadFile("./testdata/ai-coding-session-with-attribution.json")
	require.NoError(t, err)

	var evidence aicodingsession.Evidence
	require.NoError(t, json.Unmarshal(raw, &evidence))

	data := evidence.Data

	// Top-level identifiers
	assert.Equal(t, aicodingsession.EvidenceID, evidence.ID)
	assert.Equal(t, aicodingsession.EvidenceSchemaURL, evidence.Schema)
	assert.Equal(t, "v1", data.SchemaVersion)

	// Agent
	assert.Equal(t, "claude-code", data.Agent.Name)
	assert.Equal(t, "2.1.92", data.Agent.Version)

	// Session
	assert.Equal(t, "1dea3f43-4c8f-4625-8732-349613261162", data.Session.ID)
	assert.Equal(t, 14, data.Session.DurationSeconds)

	// Code changes — AI/human attribution
	require.NotNil(t, data.CodeChanges)
	assert.Equal(t, 11, data.CodeChanges.LinesAdded)
	assert.Equal(t, 8, data.CodeChanges.AILinesAdded)
	assert.Equal(t, 0, data.CodeChanges.AILinesRemoved)
	assert.Equal(t, 3, data.CodeChanges.HumanLinesAdded)
	assert.Equal(t, 0, data.CodeChanges.HumanLinesRemoved)

	// File-level fields
	require.Len(t, data.CodeChanges.Files, 2)

	goMod := data.CodeChanges.Files[0]
	assert.Equal(t, "go.mod", goMod.Path)
	assert.Equal(t, "human", goMod.Attribution)
	assert.Equal(t, 3, goMod.LinesAdded)

	mainGo := data.CodeChanges.Files[1]
	assert.Equal(t, "main.go", mainGo.Path)
	assert.Equal(t, "ai", mainGo.Attribution)
	assert.Equal(t, 8, mainGo.LinesAdded)
	require.Len(t, mainGo.LineRanges, 1)
	assert.Equal(t, 6, mainGo.LineRanges[0].Start)
	assert.Equal(t, 6, mainGo.LineRanges[0].End)
	require.Len(t, mainGo.SessionIDs, 1)
	assert.Equal(t, "1dea3f43-4c8f-4625-8732-349613261162", mainGo.SessionIDs[0])

	// Usage
	require.NotNil(t, data.Usage)
	assert.InDelta(t, 0.1917, data.Usage.EstimatedCostUSD, 0.0001)

	// Schema validation should pass
	dataBytes, err := json.Marshal(data)
	require.NoError(t, err)

	var rawData any
	require.NoError(t, json.Unmarshal(dataBytes, &rawData))
	require.NoError(t, schemavalidators.ValidateAICodingSession(rawData, schemavalidators.AICodingSessionVersion0_1))
}

func TestChainloopAICodingSessionCrafter_Annotations(t *testing.T) {
	testCases := []struct {
		name              string
		filePath          string
		expectedAgentName string
		expectedModel     string
		modelPresent      bool
	}{
		{
			name:              "full session with model",
			filePath:          "./testdata/ai-coding-session.json",
			expectedAgentName: "claude-code",
			expectedModel:     "claude-opus-4-6",
			modelPresent:      true,
		},
		{
			name:              "session with AI attribution fields",
			filePath:          "./testdata/ai-coding-session-with-attribution.json",
			expectedAgentName: "claude-code",
			expectedModel:     "claude-opus-4-6",
			modelPresent:      true,
		},
		{
			name:              "minimal session without model",
			filePath:          "./testdata/ai-coding-session-minimal.json",
			expectedAgentName: "cursor",
			modelPresent:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := zerolog.Nop()
			schema := &schemaapi.CraftingSchema_Material{
				Name: "test",
				Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION,
			}

			uploader := mUploader.NewUploader(t)
			uploader.On("UploadFile", context.TODO(), tc.filePath).
				Return(&casclient.UpDownStatus{
					Digest:   "deadbeef",
					Filename: tc.filePath,
				}, nil)

			backend := &casclient.CASBackend{Uploader: uploader}

			crafter, err := NewChainloopAICodingSessionCrafter(schema, backend, &logger)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedAgentName, got.Annotations[annotationAIAgentName])
			if tc.modelPresent {
				assert.Equal(t, tc.expectedModel, got.Annotations[annotationAICodingModel])
			} else {
				_, exists := got.Annotations[annotationAICodingModel]
				assert.False(t, exists, "model annotation should not be present")
			}
		})
	}
}
