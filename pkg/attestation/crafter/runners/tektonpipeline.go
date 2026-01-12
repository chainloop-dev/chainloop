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

package runners

import (
	"context"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
)

type TektonPipeline struct{}

func NewTektonPipeline() *TektonPipeline {
	return &TektonPipeline{}
}

func (r *TektonPipeline) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_TEKTON_PIPELINE
}

// CheckEnv detects if we're running in a Tekton environment
// by checking for the existence of Tekton-specific directories
func (r *TektonPipeline) CheckEnv() bool {
	// Check for /tekton/results directory (most reliable indicator)
	if _, err := os.Stat("/tekton/results"); err == nil {
		return true
	}
	return false
}

func (r *TektonPipeline) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{}
}

func (r *TektonPipeline) RunURI() string {
	return ""
}

func (r *TektonPipeline) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}

func (r *TektonPipeline) WorkflowFilePath() string {
	return ""
}

func (r *TektonPipeline) IsAuthenticated() bool {
	return false
}

func (r *TektonPipeline) Environment() RunnerEnvironment {
	return Unknown
}

func (r *TektonPipeline) VerifyCommitSignature(_ context.Context, _ string) *commitverification.CommitVerification {
	return nil // Not supported for this runner
}
