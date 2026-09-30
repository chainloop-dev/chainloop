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

package spec

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sessionID = "abc-123"

// writeSpec drops a spec file into a session's directory, with an mtime so
// ordering assertions do not depend on how fast the test ran.
func writeSpec(t *testing.T, repoRoot, sessionID, name, body string, modTime time.Time) string {
	t.Helper()

	require.NoError(t, EnsureDir(repoRoot))

	// The agent's file-writing tool creates the session directory; nothing in
	// this package does, so the test stands in for it.
	require.NoError(t, os.MkdirAll(SessionDir(repoRoot, sessionID), 0755))

	path := filepath.Join(SessionDir(repoRoot, sessionID), name)
	require.NoError(t, os.WriteFile(path, []byte(body), 0600))
	require.NoError(t, os.Chtimes(path, modTime, modTime))

	return path
}

func TestSessionDirStaysInsideTheSpecDirectory(t *testing.T) {
	root := t.TempDir()

	// A session ID is agent-supplied, so a path separator or a traversal in it
	// must name a directory inside ours rather than escape to anywhere else.
	for _, id := range []string{"abc-123", "../../etc/passwd", "a/b", "..", "."} {
		t.Run(id, func(t *testing.T) {
			got := SessionDir(root, id)

			assert.Equal(t, Dir(root), filepath.Dir(got), "session directory must be a direct child")
			assert.True(t, strings.HasPrefix(got, Dir(root)+string(filepath.Separator)))
		})
	}
}

func TestEnsureDir(t *testing.T) {
	t.Run("creates the tree and the ignore file", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, EnsureDir(root))

		assert.DirExists(t, Dir(root))
		// Not the per-session directory: the agent's file-writing tool makes
		// it, and pre-creating it would leave an empty one behind for every
		// session that captures nothing.
		assert.NoDirExists(t, SessionDir(root, sessionID))

		ignore, err := os.ReadFile(filepath.Join(Dir(root), ".gitignore"))
		require.NoError(t, err)
		assert.Equal(t, "*\n", string(ignore), "the directory has to ignore itself and its contents")
	})

	t.Run("is idempotent", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, EnsureDir(root))
		require.NoError(t, EnsureDir(root))
	})

	t.Run("leaves an existing ignore file alone", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, EnsureDir(root))

		path := filepath.Join(Dir(root), ".gitignore")
		require.NoError(t, os.WriteFile(path, []byte("# adjusted by hand\n*\n"), 0600))

		require.NoError(t, EnsureDir(root))

		// It is in the user's working tree and may have been changed
		// deliberately, so it is not ours to rewrite.
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "# adjusted by hand\n*\n", string(got))
	})
}

// TestSpecFilesNeverReachGit is the only assertion that actually proves the
// self-ignoring directory works: everything else just checks the file we wrote.
func TestSpecFilesNeverReachGit(t *testing.T) {
	root := t.TempDir()

	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		require.NoError(t, cmd.Run(), "git %v", args)
	}

	writeSpec(t, root, sessionID, "ticket.md", "---\nkind: ticket\n---\nbody", time.Now())

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = root
	out, err := cmd.Output()
	require.NoError(t, err)

	assert.Empty(t, string(out), "nothing under the spec directory may show up as a change")
}

func TestExists(t *testing.T) {
	root := t.TempDir()

	assert.False(t, Exists(root, sessionID), "no directory at all")

	require.NoError(t, EnsureDir(root))
	assert.False(t, Exists(root, sessionID), "the shared directory alone is not a capture")

	require.NoError(t, os.MkdirAll(SessionDir(root, sessionID), 0755))
	assert.False(t, Exists(root, sessionID), "an empty session directory is not a capture")

	writeSpec(t, root, sessionID, "ticket.md", "body", time.Now())
	assert.True(t, Exists(root, sessionID))
}

func TestReadAll(t *testing.T) {
	t.Run("no directory yields nothing", func(t *testing.T) {
		entries, warnings, err := ReadAll(t.TempDir(), sessionID)

		require.NoError(t, err)
		assert.Empty(t, entries)
		assert.Empty(t, warnings)
	})

	t.Run("orders by capture time and dates each entry from its own file", func(t *testing.T) {
		root := t.TempDir()
		second := time.Date(2026, 9, 16, 11, 0, 0, 0, time.UTC)
		first := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)

		// Written newest-first so the ordering cannot pass by accident.
		writeSpec(t, root, sessionID, "zzz-doc.md", "---\nkind: document\n---\nthe design", second)
		writeSpec(t, root, sessionID, "aaa-ticket.md", "---\nkind: ticket\nuri: https://example.com/1\n---\nthe ticket", first)

		entries, warnings, err := ReadAll(root, sessionID)

		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Empty(t, warnings)

		assert.Equal(t, aicodingsession.SpecKindTicket, entries[0].Kind)
		assert.Contains(t, string(entries[0].Raw), "the ticket")
		// The file name the agent chose is what names the stored material.
		assert.Equal(t, "aaa-ticket.md", entries[0].FileName)
		// The file as written, header included: what the push redacts and
		// keys its redacted copy on.
		assert.Equal(t, []byte("---\nkind: ticket\nuri: https://example.com/1\n---\nthe ticket"), entries[0].Raw)
		assert.Equal(t, "2026-09-16T10:00:00Z", entries[0].CapturedAt)

		assert.Equal(t, aicodingsession.SpecKindDocument, entries[1].Kind)
		assert.Equal(t, "zzz-doc.md", entries[1].FileName)
		assert.Equal(t, "2026-09-16T11:00:00Z", entries[1].CapturedAt)
	})

	t.Run("skips what is not a spec", func(t *testing.T) {
		root := t.TempDir()
		writeSpec(t, root, sessionID, "ticket.md", "the ticket", time.Now())

		// A dotfile is housekeeping, a subdirectory is not ours to walk, and an
		// empty file is what "nothing to capture" leaves behind.
		require.NoError(t, os.WriteFile(filepath.Join(SessionDir(root, sessionID), ".DS_Store"), []byte("x"), 0600))
		require.NoError(t, os.MkdirAll(filepath.Join(SessionDir(root, sessionID), "nested"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(SessionDir(root, sessionID), "empty.md"), []byte("  \n"), 0600))

		entries, warnings, err := ReadAll(root, sessionID)

		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Contains(t, string(entries[0].Raw), "the ticket")
		assert.Empty(t, warnings)
	})

	t.Run("a binary file is kept as it is, with a kind from its content", func(t *testing.T) {
		root := t.TempDir()
		// The first bytes of a PNG: enough to identify it, and not valid
		// UTF-8, as no image file is.
		pngBytes := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")
		pdfBytes := append([]byte("%PDF-1.7\n"), 0xff, 0xfe, 0x00, 0x01)
		writeSpec(t, root, sessionID, "mockup.png", string(pngBytes), time.Now())
		writeSpec(t, root, sessionID, "design.pdf", string(pdfBytes), time.Now().Add(time.Second))

		entries, warnings, err := ReadAll(root, sessionID)

		require.NoError(t, err)
		assert.Empty(t, warnings)
		require.Len(t, entries, 2)

		assert.Equal(t, "mockup.png", entries[0].FileName)
		assert.Equal(t, aicodingsession.SpecKindImage, entries[0].Kind)
		assert.True(t, entries[0].Verbatim)
		assert.Equal(t, pngBytes, entries[0].Raw, "a binary file is never rewritten")
		assert.Empty(t, entries[0].URI, "a binary file has no header to carry one")

		assert.Equal(t, aicodingsession.SpecKindDocument, entries[1].Kind)
		assert.True(t, entries[1].Verbatim)
	})

	t.Run("an image in a text format is kept as it is too", func(t *testing.T) {
		// An SVG is valid UTF-8, so only its type tells it apart from a spec
		// the agent wrote. Parsing and redacting it as text could break it.
		root := t.TempDir()
		svg := `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10"/></svg>`
		writeSpec(t, root, sessionID, "logo.svg", svg, time.Now())

		entries, warnings, err := ReadAll(root, sessionID)

		require.NoError(t, err)
		assert.Empty(t, warnings)
		require.Len(t, entries, 1)
		assert.Equal(t, aicodingsession.SpecKindImage, entries[0].Kind)
		assert.True(t, entries[0].Verbatim)
		assert.Equal(t, []byte(svg), entries[0].Raw)
	})

	t.Run("one unreadable file costs that file only, and says so", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a file whatever its mode")
		}

		root := t.TempDir()
		writeSpec(t, root, sessionID, "ticket.md", "the ticket", time.Now())
		writeSpec(t, root, sessionID, "locked.md", "the design", time.Now())
		locked := filepath.Join(SessionDir(root, sessionID), "locked.md")
		require.NoError(t, os.Chmod(locked, 0o000))
		t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })

		entries, warnings, err := ReadAll(root, sessionID)

		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Contains(t, string(entries[0].Raw), "the ticket")
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "locked.md")
		// Warnings go into the uploaded evidence, so they name the file and
		// never the local path to it.
		assert.NotContains(t, warnings[0], root)
	})

	t.Run("caps the number of entries and reports the rest", func(t *testing.T) {
		root := t.TempDir()
		base := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)

		for i := range MaxEntries + 3 {
			name := fmt.Sprintf("spec-%02d.md", i)
			writeSpec(t, root, sessionID, name, fmt.Sprintf("entry %d", i), base.Add(time.Duration(i)*time.Minute))
		}

		entries, warnings, err := ReadAll(root, sessionID)

		require.NoError(t, err)
		assert.Len(t, entries, MaxEntries)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "3 spec entries")
		// The oldest survive, so what the session started from is never the
		// thing that gets dropped.
		assert.Equal(t, "entry 0", string(entries[0].Raw))
	})
}

func TestRemove(t *testing.T) {
	root := t.TempDir()

	assert.NoError(t, Remove(root, sessionID), "a session that captured nothing is not an error")

	writeSpec(t, root, sessionID, "ticket.md", "body", time.Now())
	require.NoError(t, Remove(root, sessionID))

	assert.NoDirExists(t, SessionDir(root, sessionID))
	assert.DirExists(t, Dir(root), "other sessions keep their specs")
}

func TestRemoveDir(t *testing.T) {
	t.Run("drops the whole tree", func(t *testing.T) {
		root := t.TempDir()
		writeSpec(t, root, sessionID, "ticket.md", "body", time.Now())

		require.NoError(t, RemoveDir(root))

		assert.NoDirExists(t, filepath.Join(root, dirName))
	})

	t.Run("keeps the parent when something else lives in it", func(t *testing.T) {
		root := t.TempDir()
		writeSpec(t, root, sessionID, "ticket.md", "body", time.Now())

		other := filepath.Join(root, dirName, "something-else")
		require.NoError(t, os.WriteFile(other, []byte("not ours"), 0600))

		require.NoError(t, RemoveDir(root))

		assert.NoDirExists(t, Dir(root))
		assert.FileExists(t, other)
	})

	t.Run("nothing to remove is not an error", func(t *testing.T) {
		assert.NoError(t, RemoveDir(t.TempDir()))
	})

	t.Run("a directory it cannot look into is an error", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a directory whatever its mode")
		}

		// Reporting success here would tell the cleanup that the specs are
		// gone while they are still on disk.
		root := t.TempDir()
		writeSpec(t, root, sessionID, "ticket.md", "body", time.Now())
		require.NoError(t, os.Chmod(root, 0o000))
		t.Cleanup(func() { _ = os.Chmod(root, 0o700) })

		assert.Error(t, RemoveDir(root))
	})

	t.Run("never deletes a plain file carrying the same name", func(t *testing.T) {
		// .chainloop is also the basename of the CLI's own config file, so a
		// bare one in the working tree is not ours to remove.
		root := t.TempDir()
		notOurs := filepath.Join(root, dirName)
		require.NoError(t, os.WriteFile(notOurs, []byte("someone else's"), 0600))

		require.NoError(t, RemoveDir(root))

		assert.FileExists(t, notOurs)
	})
}
