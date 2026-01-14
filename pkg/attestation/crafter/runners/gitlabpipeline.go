//
// Copyright 2024-2026 The Chainloop Authors.
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
	"fmt"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/oidc"
	"github.com/rs/zerolog"
)

type GitlabPipeline struct {
	gitlabToken *oidc.GitlabToken
	logger      *zerolog.Logger
}

// authtoken is a possible oidc token that could be used to authenticate the runner
func NewGitlabPipeline(ctx context.Context, authToken string, logger *zerolog.Logger) *GitlabPipeline {
	client, err := oidc.NewGitlabClient(ctx, authToken, logger)
	if err != nil {
		logger.Debug().Err(err).Msgf("failed to create Gitlab OIDC client: %v", err)
		return &GitlabPipeline{
			gitlabToken: nil,
			logger:      logger,
		}
	}

	return &GitlabPipeline{
		gitlabToken: client.Token,
		logger:      logger,
	}
}

func (r *GitlabPipeline) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_GITLAB_PIPELINE
}

// Figure out if we are in a Github Action job or not
func (r *GitlabPipeline) CheckEnv() bool {
	for _, varName := range []string{"GITLAB_CI", "CI_JOB_URL"} {
		if os.Getenv(varName) == "" {
			return false
		}
	}

	return true
}

func (r *GitlabPipeline) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{
		{"GITLAB_USER_EMAIL", false},
		{"GITLAB_USER_LOGIN", false},
		{"CI_SERVER_URL", false},
		{"CI_PROJECT_URL", false},
		{"CI_COMMIT_SHA", false},
		{"CI_JOB_URL", false},
		{"CI_PIPELINE_URL", false},
		{"CI_RUNNER_VERSION", false},
		{"CI_RUNNER_DESCRIPTION", true},
		{"CI_COMMIT_REF_NAME", false},
		// MR-specific variables (optional - only present in MR contexts)
		{"CI_PIPELINE_SOURCE", true},
		{"CI_MERGE_REQUEST_IID", true},
		{"CI_MERGE_REQUEST_TITLE", true},
		{"CI_MERGE_REQUEST_DESCRIPTION", true},
		{"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", true},
		{"CI_MERGE_REQUEST_TARGET_BRANCH_NAME", true},
		{"CI_MERGE_REQUEST_PROJECT_URL", true},
	}
}

func (r *GitlabPipeline) RunURI() (url string) {
	return os.Getenv("CI_JOB_URL")
}

func (r *GitlabPipeline) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}

func (r *GitlabPipeline) WorkflowFilePath() string {
	if r.gitlabToken != nil {
		return r.gitlabToken.ConfigRefURI
	}
	return ""
}

func (r *GitlabPipeline) IsAuthenticated() bool {
	return r.gitlabToken != nil
}

func (r *GitlabPipeline) Environment() RunnerEnvironment {
	if r.gitlabToken != nil {
		switch r.gitlabToken.RunnerEnvironment {
		case "gitlab-hosted":
			return Managed
		case oidc.SelfHostedRunner:
			return SelfHosted
		default:
			return Unknown
		}
	}
	return Unknown
}

// VerifyCommitSignature checks if a commit's signature is verified by GitLab
func (r *GitlabPipeline) VerifyCommitSignature(ctx context.Context, commitHash string) *commitverification.CommitVerification {
	// Extract base URL and project path from env vars
	baseURL := os.Getenv("CI_SERVER_URL")
	projectPath := os.Getenv("CI_PROJECT_PATH")

	if baseURL == "" || projectPath == "" {
		r.logger.Debug().Msg("CI_SERVER_URL or CI_PROJECT_PATH not set, cannot verify commit")
		return nil
	}

	// Get CI_JOB_TOKEN for API access
	token := os.Getenv("CI_JOB_TOKEN")
	if token == "" {
		r.logger.Debug().Msg("CI_JOB_TOKEN not set, using unauthenticated requests")
	}

	// Call GitLab API to verify commit
	return commitverification.VerifyGitLabCommit(ctx, baseURL, projectPath, commitHash, token, r.logger)
}

// Report writes attestation table output as text artifact
func (r *GitlabPipeline) Report(tableOutput []byte) error {
	artifactFile := "chainloop-attestation-report.txt"

	if err := os.WriteFile(artifactFile, tableOutput, 0600); err != nil {
		return fmt.Errorf("failed to write attestation report: %w", err)
	}

	return nil
}
