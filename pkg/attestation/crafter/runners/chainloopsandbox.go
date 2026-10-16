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
	"github.com/chainloop-dev/chainloop/pkg/attestation/sandboxidentity"
)

const chainloopSandboxEnvVar = "CHAINLOOP_SANDBOX"

// ChainloopSandbox is the runner of a Chainloop sandbox. It is authenticated only after its
// identity token verifies, and it gives only the data of a verified identity.
type ChainloopSandbox struct {
	*Generic
	identity *sandboxidentity.Identity
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
	return os.Getenv(chainloopSandboxEnvVar) != ""
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
	return r.identity != nil
}

// IdentityToken returns the identity token that the platform gave to the sandbox, if any.
func (r *ChainloopSandbox) IdentityToken() string {
	return os.Getenv(sandboxidentity.EnvVar)
}

// SetVerifiedIdentity sets the identity of the sandbox, after the CLI and the control plane
// verified it. A nil identity makes the runner not authenticated.
func (r *ChainloopSandbox) SetVerifiedIdentity(id *sandboxidentity.Identity) {
	r.identity = id
}

func (r *ChainloopSandbox) Environment() RunnerEnvironment {
	return Unknown
}

// VerifyCommitSignature returns the commit verification of the verified identity. The sandbox has
// no SCM token, so the platform verifies the commit and signs the result in the identity. It
// returns nil without a verified identity, and for a commit other than the full commitHash.
func (r *ChainloopSandbox) VerifyCommitSignature(_ context.Context, commitHash string) *commitverification.CommitVerification {
	return r.identity.CommitVerification(commitHash)
}

func (r *ChainloopSandbox) Report(_ []byte, _ string) error {
	return nil
}
