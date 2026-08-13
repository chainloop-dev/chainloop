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
	"testing"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/storer"
	"github.com/stretchr/testify/require"
)

// TestGitStorerImplementsIdleReleaser pins the interface that
// releaseGitDescriptors depends on. That helper deliberately fails open: if a
// future go-git release stops satisfying storer.IdleReleaser it would silently
// stop releasing descriptors, reintroducing the retention regression with
// nothing failing. This assertion turns that into a build-time-visible test
// failure instead.
//
// The behavioural counterpart lives in
// TestGracefulGitRepoHeadReleasesDescriptors, which is unix-only; this test
// carries the guard on every platform.
func TestGitStorerImplementsIdleReleaser(t *testing.T) {
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	repo, err := git.PlainOpenWithOptions(repoDir, &git.PlainOpenOptions{DetectDotGit: true})
	require.NoError(t, err)

	_, ok := repo.Storer.(storer.IdleReleaser)
	require.True(t, ok,
		"go-git storer must implement storer.IdleReleaser; releaseGitDescriptors is a no-op without it")
}
