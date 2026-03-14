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
	"github.com/chainloop-dev/chainloop/internal/aiagentconfig"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChainloopAIAgentConfigCrafter_WrongType(t *testing.T) {
	logger := zerolog.Nop()

	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
	}

	_, err := NewChainloopAIAgentConfigCrafter(schema, nil, &logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "material type is not chainloop_ai_agent_config")
}

func TestNewChainloopAIAgentConfigCrafter_CorrectType(t *testing.T) {
	logger := zerolog.Nop()

	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_AGENT_CONFIG,
	}

	crafter, err := NewChainloopAIAgentConfigCrafter(schema, nil, &logger)
	require.NoError(t, err)
	assert.NotNil(t, crafter)
}

func TestChainloopAIAgentConfigCrafter_Validation(t *testing.T) {
	testCases := []struct {
		name    string
		data    *aiagentconfig.Evidence
		wantErr bool
	}{
		{
			name: "valid full config",
			data: &aiagentconfig.Evidence{
				SchemaVersion: string(schemavalidators.AIAgentConfigVersion0_1),
				Agent:         aiagentconfig.Agent{Name: "claude", Version: "4.0"},
				ConfigHash:    "abc123",
				CapturedAt:    "2026-03-13T10:00:00Z",
				GitContext: &aiagentconfig.GitContext{
					Repository: "https://github.com/org/repo",
					Branch:     "main",
					CommitSHA:  "abc123",
				},
				ConfigFiles: []aiagentconfig.ConfigFile{
					{
						Path:    "CLAUDE.md",
						SHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Size:    42,
						Content: "IyBQcm9qZWN0IFJ1bGVz",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid minimal config",
			data: &aiagentconfig.Evidence{
				SchemaVersion: string(schemavalidators.AIAgentConfigVersion0_1),
				Agent:         aiagentconfig.Agent{Name: "claude"},
				ConfigHash:    "abc123",
				CapturedAt:    "2026-03-13T10:00:00Z",
				ConfigFiles: []aiagentconfig.ConfigFile{
					{
						Path:    "CLAUDE.md",
						SHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Size:    10,
						Content: "Y29udGVudA==",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid cursor config",
			data: &aiagentconfig.Evidence{
				SchemaVersion: string(schemavalidators.AIAgentConfigVersion0_1),
				Agent:         aiagentconfig.Agent{Name: "cursor"},
				ConfigHash:    "def456",
				CapturedAt:    "2026-03-13T10:00:00Z",
				ConfigFiles: []aiagentconfig.ConfigFile{
					{
						Path:    ".cursor/rules/coding.md",
						SHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Size:    20,
						Content: "IyBDb2RpbmcgUnVsZXM=",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid cursor with multiple file types",
			data: &aiagentconfig.Evidence{
				SchemaVersion: string(schemavalidators.AIAgentConfigVersion0_1),
				Agent:         aiagentconfig.Agent{Name: "cursor"},
				ConfigHash:    "ghi789",
				CapturedAt:    "2026-03-13T10:00:00Z",
				ConfigFiles: []aiagentconfig.ConfigFile{
					{
						Path:    ".cursor/rules/react.mdc",
						SHA256:  "abc123",
						Size:    15,
						Content: "cnVsZXM=",
					},
					{
						Path:    ".cursor/agents/reviewer.md",
						SHA256:  "def456",
						Size:    10,
						Content: "YWdlbnQ=",
					},
					{
						Path:    "AGENTS.md",
						SHA256:  "789abc",
						Size:    8,
						Content: "YWdlbnRz",
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "missing required fields",
			data:    &aiagentconfig.Evidence{},
			wantErr: true,
		},
		{
			name: "empty config files",
			data: &aiagentconfig.Evidence{
				SchemaVersion: string(schemavalidators.AIAgentConfigVersion0_1),
				Agent:         aiagentconfig.Agent{Name: "claude"},
				ConfigHash:    "abc123",
				CapturedAt:    "2026-03-13T10:00:00Z",
				ConfigFiles:   []aiagentconfig.ConfigFile{},
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dataBytes, err := json.Marshal(tc.data)
			require.NoError(t, err)

			var rawData any
			require.NoError(t, json.Unmarshal(dataBytes, &rawData))

			err = schemavalidators.ValidateAIAgentConfig(rawData, schemavalidators.AIAgentConfigVersion0_1)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChainloopAIAgentConfigCrafter_InvalidJSON(t *testing.T) {
	logger := zerolog.Nop()
	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_AGENT_CONFIG,
	}

	crafter, err := NewChainloopAIAgentConfigCrafter(schema, nil, &logger)
	require.NoError(t, err)

	tmpFile := filepath.Join(t.TempDir(), "invalid.json")
	require.NoError(t, os.WriteFile(tmpFile, []byte(`{invalid json}`), 0o600))

	_, err = crafter.Craft(context.Background(), tmpFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON format")
}

func TestChainloopAIAgentConfigCrafter_InvalidSchema(t *testing.T) {
	logger := zerolog.Nop()
	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_AGENT_CONFIG,
	}

	crafter, err := NewChainloopAIAgentConfigCrafter(schema, nil, &logger)
	require.NoError(t, err)

	// Valid JSON but missing required fields
	tmpFile := filepath.Join(t.TempDir(), "bad-schema.json")
	require.NoError(t, os.WriteFile(tmpFile, []byte(`{"foo": "bar"}`), 0o600))

	_, err = crafter.Craft(context.Background(), tmpFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI agent config validation failed")
}

func TestChainloopAIAgentConfigCrafter_FileNotFound(t *testing.T) {
	logger := zerolog.Nop()
	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_AGENT_CONFIG,
	}

	crafter, err := NewChainloopAIAgentConfigCrafter(schema, nil, &logger)
	require.NoError(t, err)

	_, err = crafter.Craft(context.Background(), "/nonexistent/file.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't open the file")
}

func TestChainloopAIAgentConfigCrafter_RejectsExtraFields(t *testing.T) {
	logger := zerolog.Nop()
	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_AGENT_CONFIG,
	}

	crafter, err := NewChainloopAIAgentConfigCrafter(schema, nil, &logger)
	require.NoError(t, err)

	payload := `{
		"schema_version": "0.1",
		"agent": {"name": "claude"},
		"config_hash": "abc",
		"captured_at": "2026-03-13T10:00:00Z",
		"config_files": [{"path": "CLAUDE.md", "sha256": "abc", "size": 1, "content": "Yg=="}],
		"unknown_field": "should fail"
	}`

	tmpFile := filepath.Join(t.TempDir(), "extra-fields.json")
	require.NoError(t, os.WriteFile(tmpFile, []byte(payload), 0o600))

	_, err = crafter.Craft(context.Background(), tmpFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI agent config validation failed")
}
