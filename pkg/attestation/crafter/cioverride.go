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
	"encoding/json"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/rs/zerolog"
)

// resolveGitHubPRHeadSHA returns the actual PR branch head SHA when running
// in a GitHub Actions pull_request event.
//
// GitHub Actions creates a temporary merge commit for PR workflows, so
// .git/HEAD (and GITHUB_SHA) points to the merge commit instead of the
// actual PR head. The real SHA is available in the event payload at
// pull_request.head.sha.
//
// Note: pull_request_target is intentionally excluded because it checks out
// the base branch, not the PR branch — the PR head commit may not be
// available in the local checkout at all.
//
// Returns "" when not in a GitHub Actions PR context, or if the event
// payload is missing/unreadable.
func resolveGitHubPRHeadSHA() string {
	eventName := os.Getenv("GITHUB_EVENT_NAME")
	// Only handle pull_request events. pull_request_target checks out the
	// base branch so the PR head is unlikely to be locally available.
	if eventName != "pull_request" {
		return ""
	}

	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return ""
	}

	data, err := os.ReadFile(eventPath)
	if err != nil {
		return ""
	}

	var event struct {
		PullRequest struct {
			Head struct {
				SHA string `json:"sha"`
			} `json:"head"`
		} `json:"pull_request"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return ""
	}

	return event.PullRequest.Head.SHA
}

// overrideHeadWithPRCommit overrides headCommit's hash with the actual PR
// head SHA from the GitHub event payload. It attempts to look up the full
// commit metadata from the local repo (author, message, date). If the
// commit object is not available locally (common with shallow clones from
// actions/checkout depth=1), it still overrides the hash — which is the
// critical field for the referral graph — and keeps the existing metadata
// from the merge commit.
func overrideHeadWithPRCommit(headCommit *HeadCommit, path, actualSHA string, logger *zerolog.Logger) {
	if logger == nil {
		l := zerolog.Nop()
		logger = &l
	}

	// Try to resolve full commit metadata from the local repo
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{
		DetectDotGit:          true,
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		// Can't open repo — just override the hash
		logger.Debug().Err(err).Str("sha", actualSHA).Msg("could not open repo for PR head metadata, overriding hash only")
		headCommit.Hash = actualSHA
		return
	}

	hash := plumbing.NewHash(actualSHA)
	commit, err := repo.CommitObject(hash)
	if err != nil {
		// Commit object not available (shallow clone). Override hash, keep
		// the merge commit's metadata as best-effort.
		logger.Debug().Err(err).Str("sha", actualSHA).Msg("PR head commit not in local store (shallow clone?), overriding hash only")
		headCommit.Hash = actualSHA
		return
	}

	// Full commit available — override everything
	headCommit.Hash = commit.Hash.String()
	headCommit.AuthorEmail = commit.Author.Email
	headCommit.AuthorName = commit.Author.Name
	headCommit.Date = commit.Author.When
	headCommit.Message = commit.Message
	headCommit.Signature = commit.PGPSignature

	logger.Debug().Str("sha", actualSHA).Msg("resolved actual PR head commit instead of merge commit")
}
