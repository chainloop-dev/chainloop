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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	isPR, metadata, err := DetectPRContext(ctx, c.runner)
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
		Reviewers:    metadata.Reviewers,
	})

	jsonData, err := json.Marshal(evidenceData)
	if err != nil {
		return fmt.Errorf("marshaling PR/MR metadata: %w", err)
	}

	// Use a deterministic filename derived from a hash of the content so that
	// retries produce the same Artifact.Name (via fileStats -> os.Stat().Name())
	// and avoid duplicate CAS uploads.
	contentHash := sha256.Sum256(jsonData)
	materialName := fmt.Sprintf("pr-metadata-%s", metadata.Number)
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s.json", materialName, hex.EncodeToString(contentHash[:])))
	if err := os.WriteFile(tmpPath, jsonData, 0o600); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	defer os.Remove(tmpPath)

	if _, err := cr.AddMaterialContractFree(ctx, attestationID, schemaapi.CraftingSchema_Material_CHAINLOOP_PR_INFO.String(), materialName, tmpPath, casBackend, nil); err != nil {
		return fmt.Errorf("adding PR/MR metadata material: %w", err)
	}

	cr.Logger.Info().Msg("successfully collected and attested PR/MR metadata")

	return nil
}
