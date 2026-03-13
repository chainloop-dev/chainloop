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
	files, err := aiagentconfig.Discover(cr.WorkingDir())
	if err != nil {
		return fmt.Errorf("discovering AI agent config files: %w", err)
	}

	if len(files) == 0 {
		cr.Logger.Debug().Msg("no AI agent config files found, skipping")
		return nil
	}

	cr.Logger.Info().Int("files", len(files)).Msg("discovered AI agent config files")

	var gitCtx *aiagentconfig.GitContext
	if head := cr.CraftingState.GetAttestation().GetHead(); head != nil {
		gitCtx = &aiagentconfig.GitContext{
			CommitSHA: head.GetHash(),
		}
		if remotes := head.GetRemotes(); len(remotes) > 0 {
			gitCtx.Repository = remotes[0].GetUrl()
		}
	}

	evidence, err := aiagentconfig.BuildEvidence(cr.WorkingDir(), files, gitCtx)
	if err != nil {
		return fmt.Errorf("building AI agent config evidence: %w", err)
	}

	jsonData, err := json.Marshal(evidence)
	if err != nil {
		return fmt.Errorf("marshaling AI agent config: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "ai-agent-config-*.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(jsonData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmpFile.Close()

	if _, err := cr.AddMaterialContractFree(ctx, attestationID, "EVIDENCE", "ai-agent-config-claude", tmpFile.Name(), casBackend, nil); err != nil {
		return fmt.Errorf("adding AI agent config material: %w", err)
	}

	cr.Logger.Info().Msg("successfully collected AI agent configuration evidence")

	return nil
}
