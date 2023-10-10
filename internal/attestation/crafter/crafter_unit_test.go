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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/suite"
)

type crafterUnitSuite struct {
	suite.Suite
}

func (s *crafterUnitSuite) TestGitRepoHead() {
	initRepo := func(withCommit bool) func(string) (string, error) {
		return func(repoPath string) (string, error) {
			repo, err := git.PlainInit(repoPath, false)
			if err != nil {
				return "", err
			}

			if withCommit {
				wt, err := repo.Worktree()
				if err != nil {
					return "", err
				}

				filename := filepath.Join(repoPath, "example-git-file")
				if err = os.WriteFile(filename, []byte("hello world!"), 0600); err != nil {
					return "", err
				}

				_, err = wt.Add("example-git-file")
				if err != nil {
					return "", err
				}

				h, err := wt.Commit("test commit", &git.CommitOptions{})
				if err != nil {
					return "", err
				}

				fmt.Println("BOOOM", h)

				return h.String(), nil
			}

			return "", nil
		}
	}

	testCases := []struct {
		name          string
		repoProvider  func(string) (string, error)
		wantErr       bool
		wantEmptyHash bool
	}{
		{
			name:         "happy path",
			repoProvider: initRepo(true),
		},
		{
			name:          "empty repo",
			repoProvider:  initRepo(false),
			wantEmptyHash: true,
		},
		{
			name:          "not a repository",
			wantEmptyHash: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			path := s.T().TempDir()
			var wantDigest string
			if tc.repoProvider != nil {
				var err error
				wantDigest, err = tc.repoProvider(path)
				require.NoError(s.T(), err)
			}

			got, err := gracefulGitRepoHead(path)
			if tc.wantErr {
				assert.Error(s.T(), err)
				return
			}

			if tc.wantEmptyHash {
				assert.Empty(s.T(), got)
			} else {
				assert.NotEmpty(s.T(), got)
			}

			assert.NoError(s.T(), err)
			assert.Equal(s.T(), wantDigest, got)
		})
	}
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(crafterUnitSuite))
}
