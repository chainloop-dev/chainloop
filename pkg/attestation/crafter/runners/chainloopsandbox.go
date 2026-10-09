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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
)

const chainloopSandboxEnvVar = "CHAINLOOP_SANDBOX"

// ChainloopSandboxCommitVerificationEnvVar holds a SandboxCommitVerification as JSON.
// The sandbox gets no SCM token, so the platform verifies the commit before it starts
// the sandbox and gives the result to the runner in this variable.
const ChainloopSandboxCommitVerificationEnvVar = "CHAINLOOP_SANDBOX_COMMIT_VERIFICATION"

// SandboxCommitVerification is a commit verification result and the commit that it applies to.
type SandboxCommitVerification struct {
	// Commit is the full hash of the verified commit.
	Commit string `json:"commit"`
	// Verification is the result of the verification.
	Verification *commitverification.CommitVerification `json:"verification"`
}

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
	return false
}

func (r *ChainloopSandbox) Environment() RunnerEnvironment {
	return Unknown
}

// VerifyCommitSignature returns the verification that the platform gave in
// ChainloopSandboxCommitVerificationEnvVar. It returns nil when no verification was
// given, or when the verification is for a different commit than commitHash.
func (r *ChainloopSandbox) VerifyCommitSignature(_ context.Context, commitHash string) *commitverification.CommitVerification {
	raw := os.Getenv(ChainloopSandboxCommitVerificationEnvVar)
	if raw == "" || commitHash == "" {
		return nil
	}

	var given SandboxCommitVerification
	if err := json.Unmarshal([]byte(raw), &given); err != nil || given.Verification == nil {
		return &commitverification.CommitVerification{
			Attempted: true,
			Status:    commitverification.VerificationStatusUnavailable,
			Reason:    fmt.Sprintf("invalid value in %s", ChainloopSandboxCommitVerificationEnvVar),
		}
	}

	// The result is about one commit only. Never apply it to the commit of another
	// checkout, and never match an abbreviated hash.
	if !strings.EqualFold(given.Commit, commitHash) {
		return nil
	}

	return given.Verification
}

func (r *ChainloopSandbox) Report(_ []byte, _ string) error {
	return nil
}
