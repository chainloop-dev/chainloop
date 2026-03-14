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

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/prinfo"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
)

// PRMetadataCollector collects pull/merge request metadata from the CI environment.
type PRMetadataCollector struct {
	runner SupportedRunner
}

// NewPRMetadataCollector creates a collector that detects PR/MR context from the given runner.
func NewPRMetadataCollector(runner SupportedRunner) *PRMetadataCollector {
	return &PRMetadataCollector{runner: runner}
}

func (c *PRMetadataCollector) ID() string { return "pr-metadata" }

func (c *PRMetadataCollector) Collect(ctx context.Context, cr *Crafter, attestationID string, casBackend *casclient.CASBackend) error {
	isPR, metadata, err := DetectPRContext(c.runner)
	if err != nil {
		return fmt.Errorf("detecting PR/MR context: %w", err)
	}

	if !isPR {
		cr.Logger.Debug().Msg("not in PR/MR context, skipping metadata collection")
		return nil
	}

	cr.Logger.Info().Str("platform", metadata.Platform).Str("number", metadata.Number).Msg("detected PR/MR context")

	evidenceData := prinfo.NewEvidence(prinfo.Data{
		Platform:     metadata.Platform,
		Type:         metadata.Type,
		Number:       metadata.Number,
		Title:        metadata.Title,
		Description:  metadata.Description,
		SourceBranch: metadata.SourceBranch,
		TargetBranch: metadata.TargetBranch,
		URL:          metadata.URL,
		Author:       metadata.Author,
	})

	jsonData, err := json.Marshal(evidenceData)
	if err != nil {
		return fmt.Errorf("marshaling PR/MR metadata: %w", err)
	}

	materialName := fmt.Sprintf("pr-metadata-%s", metadata.Number)
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s-*.json", materialName))
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(jsonData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmpFile.Close()

	if _, err := cr.AddMaterialContractFree(ctx, attestationID, schemaapi.CraftingSchema_Material_CHAINLOOP_PR_INFO.String(), materialName, tmpFile.Name(), casBackend, nil); err != nil {
		return fmt.Errorf("adding PR/MR metadata material: %w", err)
	}

	cr.Logger.Info().Msg("successfully collected and attested PR/MR metadata")

	return nil
}
