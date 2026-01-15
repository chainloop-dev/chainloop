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
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/oidc"
	"github.com/rs/zerolog"
)

type GitHubAction struct {
	githubToken *oidc.Token
	logger      *zerolog.Logger
}

func NewGithubAction(ctx context.Context, logger *zerolog.Logger) *GitHubAction {
	// In order to ensure that we are running in a non-falsifiable environment we get the OIDC
	// from Github. That allows us to read the workflow file path and runnner type. If that can't
	// be done we fallback to reading the env vars directly.
	actorPersonal := fmt.Sprintf("https://github.com/%s", os.Getenv("GITHUB_ACTOR"))
	actorOrganization := fmt.Sprintf("https://github.com/%s", os.Getenv("GITHUB_REPOSITORY_OWNER"))
	client, err := oidc.NewGitHubClient(logger, oidc.WithActor(actorPersonal), oidc.WithActor(actorOrganization))
	if err != nil {
		logger.Debug().Err(err).Msg("failed creating GitHub OIDC client")
		return &GitHubAction{
			githubToken: nil,
			logger:      logger,
		}
	}

	token, err := client.Token(ctx)
	if err != nil {
		logger.Debug().Err(err).Msg("failed to get github token")
		return &GitHubAction{
			githubToken: nil,
			logger:      logger,
		}
	}

	ghToken, ok := token.(*oidc.Token)
	if !ok {
		logger.Debug().Err(err).Msg("failed casting to OIDC token")
		return &GitHubAction{
			githubToken: nil,
			logger:      logger,
		}
	}

	return &GitHubAction{
		githubToken: ghToken,
		logger:      logger,
	}
}

func (r *GitHubAction) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_GITHUB_ACTION
}

// Figure out if we are in a Github Action job or not
func (r *GitHubAction) CheckEnv() bool {
	for _, varName := range []string{"CI", "GITHUB_REPOSITORY", "GITHUB_RUN_ID"} {
		if os.Getenv(varName) == "" {
			return false
		}
	}

	return true
}

func (r *GitHubAction) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{
		{"GITHUB_ACTOR", false},
		{"GITHUB_REF", false},
		{"GITHUB_REPOSITORY", false},
		{"GITHUB_REPOSITORY_OWNER", false},
		{"GITHUB_RUN_ID", false},
		{"GITHUB_SHA", false},
		{"RUNNER_NAME", false},
		{"RUNNER_OS", false},
		// PR-specific variables (optional - only present in PR contexts)
		{"GITHUB_EVENT_NAME", true},
		{"GITHUB_HEAD_REF", true},
		{"GITHUB_BASE_REF", true},
		{"GITHUB_EVENT_PATH", true},
	}
}

func (r *GitHubAction) RunURI() (url string) {
	repo, runID := os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_RUN_ID")
	if repo != "" && runID != "" {
		url = fmt.Sprintf("https://github.com/%s/actions/runs/%s", repo, runID)
	}

	return url
}

func (r *GitHubAction) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}

func (r *GitHubAction) Environment() RunnerEnvironment {
	if r.githubToken != nil {
		switch r.githubToken.RunnerEnvironment {
		case "github-hosted":
			return Managed
		case oidc.SelfHostedRunner:
			return SelfHosted
		default:
			return Unknown
		}
	}
	return Unknown
}

func (r *GitHubAction) WorkflowFilePath() string {
	if r.githubToken != nil {
		return r.githubToken.JobWorkflowRef
	}
	return ""
}

func (r *GitHubAction) IsAuthenticated() bool {
	return r.githubToken != nil
}

// VerifyCommitSignature checks if a commit's signature is verified by GitHub
func (r *GitHubAction) VerifyCommitSignature(ctx context.Context, commitHash string) *commitverification.CommitVerification {
	// Extract owner/repo from GITHUB_REPOSITORY env var
	repo := os.Getenv("GITHUB_REPOSITORY") // e.g., "owner/repo"
	if repo == "" {
		r.logger.Debug().Msg("GITHUB_REPOSITORY not set, cannot verify commit")
		return nil
	}

	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		r.logger.Debug().Str("repo", repo).Msg("invalid GITHUB_REPOSITORY format")
		return nil
	}

	// Get GITHUB_TOKEN for API access
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		r.logger.Debug().Msg("GITHUB_TOKEN not set, API calls may be rate limited")
	}

	// Call GitHub API to verify commit
	return commitverification.VerifyGitHubCommit(ctx, parts[0], parts[1], commitHash, token, r.logger)
}

// Report writes attestation table output to GitHub Step Summary
func (r *GitHubAction) Report(tableOutput []byte) error {
	summaryFile := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryFile == "" {
		return fmt.Errorf("GITHUB_STEP_SUMMARY environment variable not set")
	}

	// Wrap table output in markdown code block
	var content strings.Builder
	content.WriteString("## Chainloop Attestation Report\n\n")
	content.WriteString("```\n")
	content.Write(tableOutput)
	content.WriteString("```\n")

	// Append to GITHUB_STEP_SUMMARY file
	f, err := os.OpenFile(summaryFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open GITHUB_STEP_SUMMARY: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content.String()); err != nil {
		return fmt.Errorf("failed to write to GITHUB_STEP_SUMMARY: %w", err)
	}

	return nil
}
