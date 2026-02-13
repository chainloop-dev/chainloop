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
	"os"
	"strings"


	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/rs/zerolog"
)

type DaggerPipeline struct {
	logger *zerolog.Logger
}

func NewDaggerPipeline(_ string, logger *zerolog.Logger) *DaggerPipeline {
	return &DaggerPipeline{
		logger: logger,
	}
}

func (r *DaggerPipeline) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_DAGGER_PIPELINE
}

func (r *DaggerPipeline) CheckEnv() bool {
	for _, envVarName := range []string{"CHAINLOOP_DAGGER_CLIENT"} {
		if os.Getenv(envVarName) == "" {
			return false
		}
	}

	return true
}

func (r *DaggerPipeline) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{
		// Version of the Chainloop Client
		{"CHAINLOOP_DAGGER_CLIENT", false},
		// Github Actions PR-specific variables (optional - only present in PR contexts)
		{"GITHUB_EVENT_NAME", true},
		{"GITHUB_HEAD_REF", true},
		{"GITHUB_BASE_REF", true},
		{"GITHUB_EVENT_PATH", true},
		// Gitlab CI MR-specific variables (optional - only present in MR contexts)
		{"CI_PIPELINE_SOURCE", true},
		{"CI_MERGE_REQUEST_IID", true},
		{"CI_MERGE_REQUEST_TITLE", true},
		{"CI_MERGE_REQUEST_DESCRIPTION", true},
		{"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", true},
		{"CI_MERGE_REQUEST_TARGET_BRANCH_NAME", true},
		{"CI_MERGE_REQUEST_PROJECT_URL", true},
		{"GITLAB_USER_LOGIN", true},
	}
}

// TODO: figure out an URL and or more useful information
func (r *DaggerPipeline) RunURI() string {
	return ""
}

func (r *DaggerPipeline) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}

func (r *DaggerPipeline) WorkflowFilePath() string {
	return ""
}

func (r *DaggerPipeline) IsAuthenticated() bool {
	return false
}

func (r *DaggerPipeline) Environment() RunnerEnvironment {
	return Unknown
}

func (r *DaggerPipeline) VerifyCommitSignature(ctx context.Context, commitHash string) *commitverification.CommitVerification {
	// Dagger can run in different CI environments. Detect which one we're in.

	// Check if running in GitHub Actions
	if r.isGitHubActionsEnvironment() {
		if r.logger != nil {
			r.logger.Debug().Msg("Dagger running in GitHub Actions, delegating verification")
		}
		return r.verifyCommitViaGitHub(ctx, commitHash)
	}

	// Check if running in GitLab CI
	if r.isGitLabCIEnvironment() {
		if r.logger != nil {
			r.logger.Debug().Msg("Dagger running in GitLab CI, delegating verification")
		}
		return r.verifyCommitViaGitLab(ctx, commitHash)
	}

	// Not running in a supported environment
	if r.logger != nil {
		r.logger.Debug().Msg("Dagger not running in GitHub Actions or GitLab CI, skipping verification")
	}
	return nil
}

// isGitHubActionsEnvironment checks if Dagger is running in GitHub Actions
func (r *DaggerPipeline) isGitHubActionsEnvironment() bool {
	// Check for GitHub Actions-specific environment variables
	return os.Getenv("GITHUB_REPOSITORY") != "" && os.Getenv("GITHUB_RUN_ID") != ""
}

// isGitLabCIEnvironment checks if Dagger is running in GitLab CI
func (r *DaggerPipeline) isGitLabCIEnvironment() bool {
	// Check for GitLab CI-specific environment variables
	return os.Getenv("GITLAB_CI") != "" && os.Getenv("CI_JOB_URL") != ""
}

// verifyCommitViaGitHub performs GitHub commit verification
func (r *DaggerPipeline) verifyCommitViaGitHub(ctx context.Context, commitHash string) *commitverification.CommitVerification {
	// Extract owner/repo from GITHUB_REPOSITORY env var
	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		if r.logger != nil {
			r.logger.Debug().Msg("GITHUB_REPOSITORY not set, cannot verify commit")
		}
		return nil
	}

	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		if r.logger != nil {
			r.logger.Debug().Str("repo", repo).Msg("invalid GITHUB_REPOSITORY format")
		}
		return nil
	}

	// Get GITHUB_TOKEN for API access
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" && r.logger != nil {
		r.logger.Debug().Msg("GITHUB_TOKEN not set, API calls may be rate limited")
	}

	// Call GitHub API to verify commit
	return commitverification.VerifyGitHubCommit(ctx, parts[0], parts[1], commitHash, token, r.logger)
}

// verifyCommitViaGitLab performs GitLab commit verification
func (r *DaggerPipeline) verifyCommitViaGitLab(ctx context.Context, commitHash string) *commitverification.CommitVerification {
	// Extract base URL and project path from env vars
	baseURL := os.Getenv("CI_SERVER_URL")
	projectPath := os.Getenv("CI_PROJECT_PATH")

	if baseURL == "" || projectPath == "" {
		if r.logger != nil {
			r.logger.Debug().Msg("CI_SERVER_URL or CI_PROJECT_PATH not set, cannot verify commit")
		}
		return nil
	}

	// Get CI_JOB_TOKEN for API access
	token := os.Getenv("CI_JOB_TOKEN")
	if token == "" && r.logger != nil {
		r.logger.Debug().Msg("CI_JOB_TOKEN not set, using unauthenticated requests")
	}

	// Call GitLab API to verify commit
	return commitverification.VerifyGitLabCommit(ctx, baseURL, projectPath, commitHash, token, r.logger)
}

func (r *DaggerPipeline) Report(_ []byte, _ string) error {
	return nil
}
