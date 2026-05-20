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

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
)

type ChainloopSandbox struct {
	*Generic
}

func NewChainloopSandbox() *ChainloopSandbox {
	return &ChainloopSandbox{
		Generic: NewGeneric(),
	}
}

func (r *ChainloopSandbox) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_CHAINLOOP_SANDBOX
}

func (r *ChainloopSandbox) CheckEnv() bool {
	return false
}

func (r *ChainloopSandbox) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{}
}

func (r *ChainloopSandbox) RunURI() string {
	return ""
}

func (r *ChainloopSandbox) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}

func (r *ChainloopSandbox) WorkflowFilePath() string {
	return ""
}

func (r *ChainloopSandbox) IsAuthenticated() bool {
	return false
}

func (r *ChainloopSandbox) Environment() RunnerEnvironment {
	return Unknown
}

func (r *ChainloopSandbox) VerifyCommitSignature(_ context.Context, _ string) *commitverification.CommitVerification {
	return nil
}

func (r *ChainloopSandbox) Report(_ []byte, _ string) error {
	return nil
}
