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

// The descriptor probe below has no Windows equivalent, and the CLI is built for
// Windows too, so this guard is scoped to unix. Linux CI and macOS development
// hosts — where descriptor limits actually bind — are both covered.
//go:build unix

package crafter

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/stretchr/testify/require"
)

// TestGitDescriptorsReleasedAtAllOpenSites guards the file-descriptor retention
// regression that go-git v6.0.0-alpha.5 introduced: every PlainOpenWithOptions
// call builds its own 256-entry descriptor pool holding roughly 3 descriptors
// per packfile (.pack/.idx/.rev), released only on LRU eviction — which never
// fires below 256 entries. Without an explicit release those descriptors stay
// open for the lifetime of the process.
//
// Every function that opens a repository must appear here: the release is a
// per-call-site defer, so a site omitted from this table can silently drop its
// release without any test failing.
//
// Each case repeats its open so a genuine leak accumulates into an unmistakable
// signal (~3 descriptors per iteration) while correct behaviour stays flat. A
// single open leaks too few descriptors to separate from incidental churn.
func TestGitDescriptorsReleasedAtAllOpenSites(t *testing.T) {
	const (
		iterations = 40
		// The assertion is "flat", not "exact". Runtime and harness activity can
		// shift the count by a few descriptors, whereas a real regression grows
		// by ~3 per iteration (~120 total).
		maxRetained = 20
	)

	repoDir, headSHA := initPackedRepo(t)

	testCases := []struct {
		name string
		// open performs one complete open-read-release cycle against the fixture.
		// It must assert that the read actually reached packed object data,
		// otherwise a short-circuiting code path would make the measurement
		// vacuous.
		open func(t *testing.T)
	}{
		{
			name: "gracefulGitRepoHead",
			open: func(t *testing.T) {
				got, err := gracefulGitRepoHead(repoDir, nil)
				require.NoError(t, err)
				require.NotNil(t, got)
				// Proves the commit object was read, not just the ref.
				require.Equal(t, headSHA, got.Hash)
				require.Equal(t, "John Doe", got.AuthorName)
			},
		},
		{
			name: "overrideHeadWithPRCommit",
			open: func(t *testing.T) {
				headCommit := &HeadCommit{}
				overrideHeadWithPRCommit(headCommit, repoDir, headSHA, nil)
				// Hash is assigned on the shallow-clone path too, so it cannot
				// distinguish a real commit read from a lookup miss. AuthorName is
				// only populated once the commit object resolves, which is what
				// puts pack descriptors in play.
				require.Equal(t, headSHA, headCommit.Hash)
				require.Equal(t, "John Doe", headCommit.AuthorName)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Warm up once so lazily-initialised state is not counted as retention.
			tc.open(t)

			before := countOpenDescriptors()
			for range iterations {
				tc.open(t)
			}
			retained := countOpenDescriptors() - before

			t.Logf("retained %d descriptors across %d opens", retained, iterations)
			require.Less(t, retained, maxRetained,
				"%s retained %d descriptors across %d opens: go-git pools packfile "+
					"descriptors per repository open and frees them only via "+
					"CloseIdleDescriptors (see releaseGitDescriptors)",
				tc.name, retained, iterations)
		})
	}
}

// initPackedRepo builds a single-commit repository whose objects live in a
// packfile, returning its path and HEAD SHA. Packing is essential rather than
// incidental: reads of loose objects never touch the pooled pack descriptors, so
// an unpacked fixture would make every retention assertion pass even with the
// regression fully present.
func initPackedRepo(t *testing.T) (repoDir, headSHA string) {
	t.Helper()

	// go-git cannot repack, so the packing step needs the git CLI.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH; needed to pack objects")
	}

	repoDir = t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	require.NoError(t, disableGPGSign(repo))

	wt, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "f.txt"), []byte("hello"), 0o600))
	_, err = wt.Add("f.txt")
	require.NoError(t, err)
	hash, err := wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "John Doe", Email: "john@doe.org", When: time.Now()},
	})
	require.NoError(t, err)

	// A GIT_DIR or GIT_WORK_TREE inherited from the ambient environment would
	// redirect gc at a different repository, leaving this fixture unpacked while
	// the command still reports success.
	cmd := exec.Command("git", "-C", repoDir, "gc", "--quiet")
	cmd.Env = slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, "GIT_DIR=") || strings.HasPrefix(kv, "GIT_WORK_TREE=")
	})
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git gc: %s", out)

	// Assert the outcome rather than trusting the command: this is what turns a
	// silently vacuous guard into a loud failure.
	packs, err := filepath.Glob(filepath.Join(repoDir, ".git", "objects", "pack", "*.pack"))
	require.NoError(t, err)
	require.NotEmpty(t, packs,
		"fixture must be packed or the descriptor guards below are vacuous")

	return repoDir, hash.String()
}

// countOpenDescriptors counts live descriptors by probing F_GETFD. /proc is
// absent on darwin and /dev/fd enumeration is unreliable there, so probe
// directly rather than reading a pseudo-filesystem.
func countOpenDescriptors() int {
	var n int
	for fd := 0; fd < 4096; fd++ {
		if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0); errno == 0 {
			n++
		}
	}
	return n
}
