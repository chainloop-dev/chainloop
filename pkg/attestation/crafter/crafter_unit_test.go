//
// Copyright 2023-2026 The Chainloop Authors.
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
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type crafterUnitSuite struct {
	suite.Suite
}

func (s *crafterUnitSuite) TestSanitizeRemoteURI() {
	testCases := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{
			name: "ssh",
			uri:  "git@cyberdyne.com:skynet.git",
			want: "git@cyberdyne.com:skynet.git",
		},
		{
			name: "https",
			uri:  "https://cyberdyne.com/skynet.git",
			want: "https://cyberdyne.com/skynet.git",
		},
		{
			name: "https with user",
			uri:  "https://demo-user:pass@cyberdyne.com/skynet.git",
			want: "https://cyberdyne.com/skynet.git",
		},
		{
			name:    "invalid uri",
			uri:     "https://demo-user@pass:cyberdyne.com/skynet.git",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			got, err := sanitizeRemoteURL(tc.uri)
			if tc.wantErr {
				s.Error(err)
				return
			}

			require.NoError(s.T(), err)
			s.Equal(tc.want, got)
		})
	}
}

func disableGPGSign(repo *git.Repository) error {
	cfg, err := repo.Config()
	if err != nil {
		return err
	}
	cfg.Commit.GpgSign = config.NewOptBool(false)
	return repo.SetConfig(cfg)
}

func (s *crafterUnitSuite) TestGitRepoHead() {
	initRepo := func(withCommit bool) func(string) (*HeadCommit, error) {
		return func(repoPath string) (*HeadCommit, error) {
			repo, err := git.PlainInit(repoPath, false)
			if err != nil {
				return nil, err
			}

			if err := disableGPGSign(repo); err != nil {
				return nil, err
			}

			_, err = repo.CreateRemote(&config.RemoteConfig{
				Name: "origin",
				URLs: []string{"git@cyberdyne.com:skynet.git"},
			})

			if err != nil {
				return nil, err
			}

			if withCommit {
				wt, err := repo.Worktree()
				if err != nil {
					return nil, err
				}

				filename := filepath.Join(repoPath, "example-git-file")
				if err = os.WriteFile(filename, []byte("hello world!"), 0600); err != nil {
					return nil, err
				}

				_, err = wt.Add("example-git-file")
				if err != nil {
					return nil, err
				}

				h, err := wt.Commit("test commit", &git.CommitOptions{
					Author: &object.Signature{
						Name:  "John Doe",
						Email: "john@doe.org",
						When:  time.Now(),
					},
				})
				if err != nil {
					return nil, err
				}

				return &HeadCommit{
					Hash:        h.String(),
					AuthorEmail: "john@doe.org",
					AuthorName:  "John Doe",
					Message:     "test commit",
				}, nil
			}

			return nil, nil
		}
	}

	testCases := []struct {
		name         string
		repoProvider func(string) (*HeadCommit, error)
		wantErr      bool
		wantNoCommit bool
	}{
		{
			name:         "happy path",
			repoProvider: initRepo(true),
		},
		{
			name:         "empty repo",
			repoProvider: initRepo(false),
			wantNoCommit: true,
		},
		{
			name:         "not a repository",
			wantNoCommit: true,
		},
		{
			name: "repo with unsupported extension degrades gracefully",
			repoProvider: func(repoPath string) (*HeadCommit, error) {
				// Init a repo and add a worktreeConfig extension to trigger
				// go-git's strict extension validation (added in v5.17.0)
				if _, err := git.PlainInit(repoPath, false); err != nil {
					return nil, err
				}

				// Write the extension directly into the git config file
				gitConfigPath := filepath.Join(repoPath, ".git", "config")
				f, err := os.OpenFile(gitConfigPath, os.O_APPEND|os.O_WRONLY, 0o600)
				if err != nil {
					return nil, err
				}
				defer f.Close()
				if _, err := f.WriteString("[extensions]\n\tworktreeConfig = true\n"); err != nil {
					return nil, err
				}

				return nil, nil
			},
			wantNoCommit: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			path := s.T().TempDir()
			var wantCommit *HeadCommit
			if tc.repoProvider != nil {
				var err error
				wantCommit, err = tc.repoProvider(path)
				require.NoError(s.T(), err)
			}

			got, err := gracefulGitRepoHead(path, nil)
			if tc.wantErr {
				assert.Error(s.T(), err)
				return
			}

			require.NoError(s.T(), err)

			if tc.wantNoCommit {
				assert.Empty(s.T(), got)
				return
			}

			assert.Equal(s.T(), wantCommit.AuthorEmail, got.AuthorEmail)
			assert.Equal(s.T(), wantCommit.AuthorName, got.AuthorName)
			assert.Equal(s.T(), wantCommit.Hash, got.Hash)
			assert.NotEmpty(s.T(), got.Remotes)
			assert.Equal(s.T(), &CommitRemote{
				Name: "origin",
				URL:  "git@cyberdyne.com:skynet.git",
			}, got.Remotes[0])
			assert.NotEmpty(s.T(), got.Date)
		})
	}
}

func (s *crafterUnitSuite) TestPolicyEvaluationDedup() {
	// Simulate the protojson round-trip issue:
	// - Init phase sets With = map[string]string{} (empty map)
	// - protojson.Marshal omits empty maps
	// - protojson.Unmarshal sets With = nil (absent field)
	// - Push phase produces With = map[string]string{} again
	// The dedup comparison must treat nil and empty map as equal.

	policyRef := &api.PolicyEvaluation_Reference{
		Name:   "source-commit",
		Digest: "sha256:abc123",
	}

	testCases := []struct {
		name        string
		existing    []*api.PolicyEvaluation
		newEvals    []*api.PolicyEvaluation
		wantCount   int
		description string
	}{
		{
			name: "nil vs empty map With are deduplicated",
			existing: []*api.PolicyEvaluation{
				{Name: "source-commit", PolicyReference: policyRef, With: nil},
			},
			newEvals: []*api.PolicyEvaluation{
				{Name: "source-commit", PolicyReference: policyRef, With: map[string]string{}},
			},
			wantCount:   1,
			description: "after protojson round-trip, nil With should match empty map With",
		},
		{
			name: "empty map vs empty map With are deduplicated",
			existing: []*api.PolicyEvaluation{
				{Name: "source-commit", PolicyReference: policyRef, With: map[string]string{}},
			},
			newEvals: []*api.PolicyEvaluation{
				{Name: "source-commit", PolicyReference: policyRef, With: map[string]string{}},
			},
			wantCount:   1,
			description: "identical empty maps should deduplicate",
		},
		{
			name: "nil vs nil With are deduplicated",
			existing: []*api.PolicyEvaluation{
				{Name: "source-commit", PolicyReference: policyRef, With: nil},
			},
			newEvals: []*api.PolicyEvaluation{
				{Name: "source-commit", PolicyReference: policyRef, With: nil},
			},
			wantCount:   1,
			description: "both nil should deduplicate",
		},
		{
			name: "different With args are not deduplicated",
			existing: []*api.PolicyEvaluation{
				{Name: "source-commit", PolicyReference: policyRef, With: map[string]string{"key": "val1"}},
			},
			newEvals: []*api.PolicyEvaluation{
				{Name: "source-commit", PolicyReference: policyRef, With: map[string]string{"key": "val2"}},
			},
			wantCount:   2,
			description: "different With values should not deduplicate",
		},
		{
			name: "different policy references are not deduplicated",
			existing: []*api.PolicyEvaluation{
				{Name: "policy-a", PolicyReference: &api.PolicyEvaluation_Reference{Name: "policy-a"}, With: nil},
			},
			newEvals: []*api.PolicyEvaluation{
				{Name: "policy-b", PolicyReference: &api.PolicyEvaluation_Reference{Name: "policy-b"}, With: nil},
			},
			wantCount:   2,
			description: "different policies should both be kept",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			all := slices.Concat(tc.existing, tc.newEvals)

			var filtered []*api.PolicyEvaluation
			for _, ev := range all {
				var duplicated bool
				for _, existing := range filtered {
					if policyEvalMatches(existing, ev) {
						duplicated = true
						break
					}
				}
				if !duplicated {
					filtered = append(filtered, ev)
				}
			}

			s.Len(filtered, tc.wantCount, tc.description)
		})
	}
}

func (s *crafterUnitSuite) TestGitRepoHeadWorktree() {
	// go-git cannot create worktrees, so use the git CLI
	if _, err := exec.LookPath("git"); err != nil {
		s.T().Skip("git not found in PATH")
	}

	repoPath := s.T().TempDir()

	// Initialize a repo and create a commit using go-git
	repo, err := git.PlainInit(repoPath, false)
	require.NoError(s.T(), err)

	require.NoError(s.T(), disableGPGSign(repo))

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"git@cyberdyne.com:skynet.git"},
	})
	require.NoError(s.T(), err)

	wt, err := repo.Worktree()
	require.NoError(s.T(), err)

	filename := filepath.Join(repoPath, "example-git-file")
	require.NoError(s.T(), os.WriteFile(filename, []byte("hello world!"), 0o600))

	_, err = wt.Add("example-git-file")
	require.NoError(s.T(), err)

	h, err := wt.Commit("test commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	})
	require.NoError(s.T(), err)

	// Create a worktree using git CLI (go-git doesn't support this)
	worktreePath := filepath.Join(s.T().TempDir(), "test-worktree")
	cmd := exec.Command("git", "-C", repoPath, "worktree", "add", worktreePath)
	out, err := cmd.CombinedOutput()
	require.NoError(s.T(), err, "git worktree add: %s", out)

	got, err := gracefulGitRepoHead(worktreePath, nil)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), got)

	assert.Equal(s.T(), h.String(), got.Hash)
	assert.Equal(s.T(), "john@doe.org", got.AuthorEmail)
	assert.Equal(s.T(), "John Doe", got.AuthorName)
	assert.NotEmpty(s.T(), got.Remotes)
	assert.Equal(s.T(), &CommitRemote{
		Name: "origin",
		URL:  "git@cyberdyne.com:skynet.git",
	}, got.Remotes[0])
	assert.NotEmpty(s.T(), got.Date)
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(crafterUnitSuite))
}
