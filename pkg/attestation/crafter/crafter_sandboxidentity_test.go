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

package crafter_test

import (
	"context"
	"fmt"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/chainloop-dev/chainloop/pkg/attestation/sandboxidentity"
	"github.com/rs/zerolog"
)

// The verified identity of the sandbox runner must reach the persisted attestation state: an
// authenticated runner and the commit verification of git.head (R-004, R-005).
func (s *crafterSuite) TestInitPersistsTheSandboxIdentity() {
	s.T().Setenv("CHAINLOOP_SANDBOX", "1")

	verifiedIdentity := &sandboxidentity.Identity{Claims: &sandboxidentity.Claims{Data: &sandboxidentity.Data{
		CommitVerification: &sandboxidentity.CommitVerification{
			Commit:       s.repoHead,
			Verification: &commitverification.CommitVerification{Attempted: true, Status: commitverification.VerificationStatusVerified, Platform: "github"},
		},
	}}}

	testCases := []struct {
		name     string
		identity *sandboxidentity.Identity
		wantAuth bool
	}{
		{name: "with a verified identity", identity: verifiedIdentity, wantAuth: true},
		{name: "without a verified identity"},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			contract, err := loadSchema("testdata/contracts/empty_generic.yaml")
			s.Require().NoError(err)

			statePath := fmt.Sprintf("%s/attestation.json", s.T().TempDir())
			c, err := crafter.NewCrafter(testingStateManager(s.T(), statePath), nil, crafter.WithWorkingDirPath(s.repoPath))
			s.Require().NoError(err)

			runner := runners.NewChainloopSandbox()
			runner.SetVerifiedIdentity(tc.identity)

			ctx := context.Background()
			s.Require().NoError(c.Init(ctx, &crafter.InitOpts{SchemaV1: contract, WfInfo: s.workflowMetadata, DryRun: true, Runner: runner}))
			s.Require().NoError(c.ResolveEnvVars(ctx, ""))

			// Read the state back as the next commands of the attestation do
			logger := zerolog.Nop()
			loaded, err := crafter.NewCrafter(testingStateManager(s.T(), statePath), nil, crafter.WithLogger(&logger))
			s.Require().NoError(err)
			s.Require().NoError(loaded.LoadCraftingState(ctx, ""))

			att := loaded.CraftingState.GetAttestation()
			s.Equal(schemaapi.CraftingSchema_Runner_CHAINLOOP_SANDBOX, att.GetRunnerType())
			s.Equal(tc.wantAuth, att.GetRunnerEnvironment().GetAuthenticated())

			verification := att.GetHead().GetPlatformVerification()
			if !tc.wantAuth {
				s.Nil(verification)
				return
			}

			s.Require().NotNil(verification)
			s.Equal(v1.Commit_CommitVerification_verified, verification.GetStatus())
		})
	}
}
