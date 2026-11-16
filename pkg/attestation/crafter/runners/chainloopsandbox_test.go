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
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/chainloop-dev/chainloop/pkg/attestation/sandboxidentity"
	"github.com/stretchr/testify/suite"
)

type chainloopSandboxSuite struct {
	suite.Suite
	runner *ChainloopSandbox
}

func (s *chainloopSandboxSuite) TestCheckEnv() {
	testCases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			name: "env var not set",
			env:  map[string]string{},
			want: false,
		},
		{
			name: "env var set to non-empty value",
			env:  map[string]string{"CHAINLOOP_SANDBOX": "1"},
			want: true,
		},
		{
			name: "env var set to empty string",
			env:  map[string]string{"CHAINLOOP_SANDBOX": ""},
			want: false,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			t.Setenv("CHAINLOOP_SANDBOX", "")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			s.Equal(tc.want, s.runner.CheckEnv())
		})
	}
}

func (s *chainloopSandboxSuite) TestListEnvVars() {
	s.Empty(s.runner.ListEnvVars())
}

func (s *chainloopSandboxSuite) TestResolveEnvVars() {
	resolved, errors := s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Empty(resolved)
}

func (s *chainloopSandboxSuite) TestRunURI() {
	s.Empty(s.runner.RunURI())
}

func (s *chainloopSandboxSuite) TestWorkflowFilePath() {
	s.Empty(s.runner.WorkflowFilePath())
}

func (s *chainloopSandboxSuite) TestIsAuthenticated() {
	s.False(s.runner.IsAuthenticated())

	s.runner.SetVerifiedIdentity(&sandboxidentity.Identity{Claims: &sandboxidentity.Claims{}})
	s.True(s.runner.IsAuthenticated())

	s.runner.SetVerifiedIdentity(nil)
	s.False(s.runner.IsAuthenticated())
}

func (s *chainloopSandboxSuite) TestIdentityToken() {
	s.T().Setenv(sandboxidentity.EnvVar, "")
	s.Empty(s.runner.IdentityToken())

	s.T().Setenv(sandboxidentity.EnvVar, "a.b.c")
	s.Equal("a.b.c", s.runner.IdentityToken())
}

func (s *chainloopSandboxSuite) TestEnvironment() {
	s.Equal(Unknown, s.runner.Environment())
}

func (s *chainloopSandboxSuite) TestRunnerName() {
	s.Equal("CHAINLOOP_SANDBOX", s.runner.ID().String())
}

func (s *chainloopSandboxSuite) TestVerifyCommitSignature() {
	const (
		commit      = "ea3a974660a030f0314aa1505a64792bf2633b33"
		otherCommit = "0d7bb4b1e7b3f5d3e5c5f0f37a2c9a1f7f8b9c10"
	)

	verified := &commitverification.CommitVerification{
		Attempted:          true,
		Status:             commitverification.VerificationStatusVerified,
		Reason:             "Commit signed and verified",
		Platform:           "gitlab",
		KeyID:              "1",
		SignatureAlgorithm: "PGP",
	}

	withData := &sandboxidentity.Identity{Claims: &sandboxidentity.Claims{Data: &sandboxidentity.Data{
		CommitVerification: &sandboxidentity.CommitVerification{Commit: commit, Verification: verified},
	}}}

	testCases := []struct {
		name     string
		identity *sandboxidentity.Identity
		commit   string
		want     *commitverification.CommitVerification
	}{
		{name: "no verified identity", commit: commit},
		{name: "the verification of the attested commit", identity: withData, commit: commit, want: verified},
		{name: "a commit hash in upper case", identity: withData, commit: strings.ToUpper(commit), want: verified},
		{name: "the verification of another commit", identity: withData, commit: otherCommit},
		{name: "an abbreviated commit hash", identity: withData, commit: commit[:7]},
		{name: "no commit to verify", identity: withData, commit: ""},
		{name: "an identity without data", identity: &sandboxidentity.Identity{Claims: &sandboxidentity.Claims{}}, commit: commit},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			// A value set by hand, as the unsigned hand-off did, records no data (R-005)
			t.Setenv("CHAINLOOP_SANDBOX_COMMIT_VERIFICATION", `{"commit":"`+commit+`","verification":{"attempted":true,"status":"verified"}}`)

			runner := NewChainloopSandbox()
			runner.SetVerifiedIdentity(tc.identity)
			s.Equal(tc.want, runner.VerifyCommitSignature(context.Background(), tc.commit))
		})
	}
}

func (s *chainloopSandboxSuite) SetupTest() {
	s.runner = NewChainloopSandbox()
}

func TestChainloopSandboxRunner(t *testing.T) {
	suite.Run(t, new(chainloopSandboxSuite))
}
