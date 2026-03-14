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

package crafter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/aiagentconfig"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
)

// AIAgentConfigCollector discovers AI agent configuration files and attaches them as evidence.
type AIAgentConfigCollector struct{}

// NewAIAgentConfigCollector creates a new AI agent config collector.
func NewAIAgentConfigCollector() *AIAgentConfigCollector {
	return &AIAgentConfigCollector{}
}

func (c *AIAgentConfigCollector) ID() string { return "ai-agent-config" }

func (c *AIAgentConfigCollector) Collect(ctx context.Context, cr *Crafter, attestationID string, casBackend *casclient.CASBackend) error {
	agentFiles, err := aiagentconfig.DiscoverAll(cr.WorkingDir())
	if err != nil {
		return fmt.Errorf("discovering AI agent config files: %w", err)
	}

	if len(agentFiles) == 0 {
		cr.Logger.Debug().Msg("no AI agent config files found, skipping")
		return nil
	}

	var gitCtx *aiagentconfig.GitContext
	if head := cr.CraftingState.GetAttestation().GetHead(); head != nil {
		gitCtx = &aiagentconfig.GitContext{
			CommitSHA: head.GetHash(),
		}
		if remotes := head.GetRemotes(); len(remotes) > 0 {
			gitCtx.Repository = remotes[0].GetUrl()
		}
	}

	// Process each agent in deterministic order
	agentNames := make([]string, 0, len(agentFiles))
	for name := range agentFiles {
		agentNames = append(agentNames, name)
	}
	sort.Strings(agentNames)

	for _, agentName := range agentNames {
		files := agentFiles[agentName]

		cr.Logger.Info().Str("agent", agentName).Int("files", len(files)).Msg("discovered AI agent config files")
		cr.Logger.Debug().Str("agent", agentName).Strs("paths", files).Msg("AI agent config file paths")

		if err := c.uploadAgentConfig(ctx, cr, attestationID, casBackend, agentName, files, gitCtx); err != nil {
			return err
		}
	}

	return nil
}

func (c *AIAgentConfigCollector) uploadAgentConfig(
	ctx context.Context, cr *Crafter, attestationID string,
	casBackend *casclient.CASBackend, agentName string, files []string, gitCtx *aiagentconfig.GitContext,
) error {
	evidence, err := aiagentconfig.Build(cr.WorkingDir(), files, agentName, gitCtx)
	if err != nil {
		return fmt.Errorf("building AI agent config for %s: %w", agentName, err)
	}

	jsonData, err := json.Marshal(evidence)
	if err != nil {
		return fmt.Errorf("marshaling AI agent config for %s: %w", agentName, err)
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("ai-agent-config-%s-*.json", agentName))
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(jsonData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmpFile.Close()

	materialName := fmt.Sprintf("ai-agent-config-%s", agentName)
	if _, err := cr.AddMaterialContractFree(ctx, attestationID, schemaapi.CraftingSchema_Material_CHAINLOOP_AI_AGENT_CONFIG.String(), materialName, tmpFile.Name(), casBackend, nil); err != nil {
		return fmt.Errorf("adding AI agent config material for %s: %w", agentName, err)
	}

	cr.Logger.Info().Str("agent", agentName).Msg("successfully collected AI agent configuration")

	return nil
}
