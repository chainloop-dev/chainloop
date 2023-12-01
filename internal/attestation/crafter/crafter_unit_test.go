//
// Copyright 2023 The Chainloop Authors.
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
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-git/go-git/v5/config"
	"github.com/stretchr/testify/suite"
)

type crafterUnitSuite struct {
	suite.Suite
}

func (s *crafterUnitSuite) TestGitRepoHead() {
	initRepo := func(withCommit bool) func(string) (*HeadCommit, error) {
		return func(repoPath string) (*HeadCommit, error) {
			repo, err := git.PlainInit(repoPath, false)
			if err != nil {
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

			got, err := NewCrafter().gracefulGitRepoHead(path)
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

func TestSuite(t *testing.T) {
	suite.Run(t, new(crafterUnitSuite))
}
