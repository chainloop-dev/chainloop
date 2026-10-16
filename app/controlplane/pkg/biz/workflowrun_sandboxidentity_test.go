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

package biz

import (
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/attestation/sandboxidentity"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestValidateSandboxRunner(t *testing.T) {
	const (
		commit       = "0123456789abcdef0123456789abcdef01234567"
		sandbox      = "CHAINLOOP_SANDBOX"
		github       = "GITHUB_ACTION"
		verified     = "verified"
		commitClaims = "commit verification"
	)

	identity := &SandboxIdentity{Data: &sandboxidentity.Data{CommitVerification: &sandboxidentity.CommitVerification{
		Commit:       commit,
		Verification: &commitverification.CommitVerification{Attempted: true, Status: commitverification.VerificationStatusVerified},
	}}}

	withAlgorithm := &SandboxIdentity{Data: &sandboxidentity.Data{CommitVerification: &sandboxidentity.CommitVerification{
		Commit:       commit,
		Verification: &commitverification.CommitVerification{Attempted: true, Status: commitverification.VerificationStatusVerified, SignatureAlgorithm: "PGP"},
	}}}

	// statement records a verification of the commit, and its algorithm after a "/" in status
	statement := func(t *testing.T, commit, status string) *intoto.Statement {
		t.Helper()
		if commit == "" {
			return &intoto.Statement{}
		}

		fields := map[string]any{"author.verification_status": status}
		if name, algorithm, ok := strings.Cut(status, "/"); ok {
			fields = map[string]any{"author.verification_status": name, "signature.algorithm": algorithm}
		}

		annotations, err := structpb.NewStruct(fields)
		require.NoError(t, err)

		return &intoto.Statement{Subject: []*intoto.ResourceDescriptor{
			{Name: chainloop.SubjectGitHead, Digest: map[string]string{"sha1": commit}, Annotations: annotations},
		}}
	}

	predicate := func(runnerType string, authenticated bool) chainloop.NormalizablePredicate {
		return &chainloop.ProvenancePredicateV02{ProvenancePredicateCommon: &chainloop.ProvenancePredicateCommon{
			RunnerType: runnerType, RunnerAuthenticated: authenticated,
		}}
	}

	testCases := []struct {
		name            string
		runRunner       string
		identity        *SandboxIdentity
		attestedRunner  string
		authenticated   bool
		commit, status  string
		wantErrContains string
	}{
		{name: "a CI runner keeps its behavior", runRunner: github, attestedRunner: github, authenticated: true, commit: commit, status: verified},
		{name: "a sandbox runner that claims nothing", runRunner: sandbox, attestedRunner: sandbox},
		{name: "an authenticated sandbox runner with a verified identity", runRunner: sandbox, identity: identity, attestedRunner: sandbox, authenticated: true, commit: commit, status: verified},
		{name: "an authenticated sandbox runner without an identity", runRunner: sandbox, attestedRunner: sandbox, authenticated: true, wantErrContains: "authenticated"},
		{name: "a commit verification without an identity", runRunner: sandbox, attestedRunner: sandbox, commit: commit, status: verified, wantErrContains: commitClaims},
		{name: "a commit verification for another commit", runRunner: sandbox, identity: identity, attestedRunner: sandbox, authenticated: true, commit: "fedcba9876543210fedcba9876543210fedcba98", status: verified, wantErrContains: commitClaims},
		{name: "a commit verification with another status", runRunner: sandbox, identity: identity, attestedRunner: sandbox, authenticated: true, commit: commit, status: "unverified", wantErrContains: commitClaims},
		{name: "a sandbox run that attests another runner", runRunner: sandbox, attestedRunner: github, authenticated: true, wantErrContains: "authenticated"},
		{name: "an attestation that claims the sandbox runner in another run", runRunner: github, attestedRunner: sandbox, commit: commit, status: verified, wantErrContains: commitClaims},
		{name: "the signature algorithm of the identity", runRunner: sandbox, identity: withAlgorithm, attestedRunner: sandbox, authenticated: true, commit: commit, status: verified + "/PGP"},
		{name: "another signature algorithm than the identity", runRunner: sandbox, identity: withAlgorithm, attestedRunner: sandbox, authenticated: true, commit: commit, status: verified + "/SSH", wantErrContains: commitClaims},
		{name: "no signature algorithm, but the identity has one", runRunner: sandbox, identity: withAlgorithm, attestedRunner: sandbox, authenticated: true, commit: commit, status: verified, wantErrContains: commitClaims},
		{name: "an identity without an algorithm accepts the one of the commit signature", runRunner: sandbox, identity: identity, attestedRunner: sandbox, authenticated: true, commit: commit, status: verified + "/SSH"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run := &WorkflowRun{RunnerType: tc.runRunner, SandboxIdentity: tc.identity}

			err := validateSandboxRunner(run, statement(t, tc.commit, tc.status), predicate(tc.attestedRunner, tc.authenticated))
			if tc.wantErrContains == "" {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.True(t, IsErrValidation(err))
			assert.Contains(t, err.Error(), tc.wantErrContains)
		})
	}
}
