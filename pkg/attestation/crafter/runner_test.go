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
	"bytes"
	"context"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Environments that make the CheckEnv of a runner match
var (
	gitlabEnv = map[string]string{"GITLAB_CI": "true", "CI_JOB_URL": "https://gitlab.example.com/group/project/-/jobs/1"}
	githubEnv = map[string]string{"CI": "true", "GITHUB_REPOSITORY": "owner/repo", "GITHUB_RUN_ID": "1"}
	daggerEnv = map[string]string{"CHAINLOOP_DAGGER_CLIENT": "v1.0.0"}
	// otherRunnerEnvVars are read by the CheckEnv of the remaining runners
	otherRunnerEnvVars = []string{"TF_BUILD", "BUILD_BUILDURI", "JENKINS_HOME", "BUILD_URL", "CIRCLECI", "TEAMCITY_PROJECT_NAME", "CHAINLOOP_SANDBOX"}
)

func TestDiscoverRunner(t *testing.T) {
	testCases := []struct {
		name string
		envs []map[string]string
		want schemaapi.CraftingSchema_Runner_RunnerType
	}{
		{
			name: "no runner environment",
			want: schemaapi.CraftingSchema_Runner_RUNNER_TYPE_UNSPECIFIED,
		},
		{
			name: "gitlab pipeline",
			envs: []map[string]string{gitlabEnv},
			want: schemaapi.CraftingSchema_Runner_GITLAB_PIPELINE,
		},
		{
			name: "github action",
			envs: []map[string]string{githubEnv},
			want: schemaapi.CraftingSchema_Runner_GITHUB_ACTION,
		},
		{
			// The Chainloop Dagger module passes the parent GitLab CI context to the CLI container
			name: "dagger pipeline with gitlab context",
			envs: []map[string]string{daggerEnv, gitlabEnv},
			want: schemaapi.CraftingSchema_Runner_DAGGER_PIPELINE,
		},
		{
			name: "dagger pipeline with github context",
			envs: []map[string]string{daggerEnv, githubEnv},
			want: schemaapi.CraftingSchema_Runner_DAGGER_PIPELINE,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear the runner variables of the environment the test runs in, for example a CI job
			for _, env := range []map[string]string{gitlabEnv, githubEnv, daggerEnv} {
				for name := range env {
					t.Setenv(name, "")
				}
			}
			for _, name := range otherRunnerEnvVars {
				t.Setenv(name, "")
			}
			for _, env := range tc.envs {
				for name, value := range env {
					t.Setenv(name, value)
				}
			}

			// Discover several times: map iteration order is random, so a
			// single call can pick the expected runner by chance
			for range 50 {
				assert.Equal(t, tc.want, DiscoverRunner("", zerolog.Nop()).ID())
			}
		})
	}
}

// verifyingRunner is a runner that returns a fixed commit verification result
type verifyingRunner struct {
	*runners.Generic
	verification *commitverification.CommitVerification
}

func (r *verifyingRunner) VerifyCommitSignature(_ context.Context, _ string) *commitverification.CommitVerification {
	return r.verification
}

func TestVerifyCommitWithPlatformLogsUnavailableReason(t *testing.T) {
	testCases := []struct {
		name         string
		verification *commitverification.CommitVerification
		wantStatus   api.Commit_CommitVerification_VerificationStatus
		wantWarning  bool
	}{
		{
			name: "unavailable",
			verification: &commitverification.CommitVerification{
				Attempted: true,
				Status:    commitverification.VerificationStatusUnavailable,
				Reason:    "GitLab API error: x509: certificate signed by unknown authority",
				Platform:  "gitlab",
			},
			wantStatus:  api.Commit_CommitVerification_unavailable,
			wantWarning: true,
		},
		{
			name: "verified",
			verification: &commitverification.CommitVerification{
				Attempted: true,
				Status:    commitverification.VerificationStatusVerified,
				Reason:    "Commit signed and verified",
				Platform:  "gitlab",
			},
			wantStatus: api.Commit_CommitVerification_verified,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := zerolog.New(&logs)
			runner := &verifyingRunner{Generic: runners.NewGeneric(), verification: tc.verification}

			got := verifyCommitWithPlatform(&HeadCommit{Hash: "abc"}, runner, &logger)

			require.NotNil(t, got)
			assert.Equal(t, tc.wantStatus, got.GetStatus())
			if tc.wantWarning {
				assert.Contains(t, logs.String(), `"level":"warn"`)
				assert.Contains(t, logs.String(), tc.verification.Reason)
			} else {
				assert.Empty(t, logs.String())
			}
		})
	}
}
