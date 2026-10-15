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

package git

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGoGitClient_SnapshotIndex_ResolutionChangesSignature(t *testing.T) {
	// Given
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))

		return strings.TrimSpace(string(out))
	}
	writeIndex := func(entries string) {
		t.Helper()
		cmd := exec.Command("git", "update-index", "--index-info")
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(entries)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git update-index: %s", string(out))
	}
	writeObject := func(content string) string {
		t.Helper()
		cmd := exec.Command("git", "hash-object", "-w", "--stdin")
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(content)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git hash-object: %s", string(out))

		return strings.TrimSpace(string(out))
	}

	run("init", "-q")
	run("read-tree", "--empty")
	base := writeObject("base\n")
	ours := writeObject("ours\n")
	theirs := writeObject("theirs\n")
	writeIndex(fmt.Sprintf(
		"100644 %s 1\tconflict.txt\n100644 %s 2\tconflict.txt\n100644 %s 3\tconflict.txt\n",
		base, ours, theirs,
	))

	client := NewGoGitClient()
	before, err := client.SnapshotIndex(dir)
	require.NoError(t, err)

	run("read-tree", "--empty")
	writeIndex(fmt.Sprintf("100644 %s 0\tconflict.txt\n", theirs))

	// When
	after, err := client.SnapshotIndex(dir)
	require.NoError(t, err)

	// Then
	require.Contains(t, before, "conflict.txt")
	require.Contains(t, after, "conflict.txt")
	require.NotEqual(t, before["conflict.txt"], after["conflict.txt"],
		"resolving to an existing conflict stage must change the index signature")
}
