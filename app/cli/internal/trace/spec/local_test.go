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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSource writes a local source file at path with a known modification
// time, creating its parent directory.
func writeSource(t *testing.T, path, content string, modTime time.Time) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	require.NoError(t, os.Chtimes(path, modTime, modTime))
}

// TestReadAllLocalSource covers spec issue-3561: a capture whose source is a
// local file is recorded with the content of that file at push time.
func TestReadAllLocalSource(t *testing.T) {
	const (
		current   = "# Spec foo\n\nThe last edit.\n"
		header    = "---\nkind: document\nuri: %s\nrole: spec\ntitle: \"Spec foo\"\n---\n"
		fooPath   = "docs/specs/foo.md"
		gonePath  = "docs/specs/gone.md"
		bigPath   = "big.md"
		agentBody = "the copy the agent wrote\n"
		ticketURL = "https://tracker.example.com/issue/1"
	)

	// uri returns a fixed uri, whatever the repository root.
	uri := func(u string) func(_, _ string) string { return func(_, _ string) string { return u } }

	captureTime := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	fileTime := time.Date(2026, 10, 2, 15, 30, 0, 0, time.UTC)
	largeContent := strings.Repeat("a", maxLocalSourceSize+1)
	binaryContent := "\x00\x01\xff\xfe not text"

	testCases := []struct {
		name string
		// uri returns the uri of the capture, given the repository root and
		// the home directory of the test.
		uri func(root, home string) string
		// source is where the local file is written, relative to the
		// repository root, or empty for no file.
		source        string
		sourceContent string
		// sourceDir makes the source a directory rather than a file.
		sourceDir bool
		body      string
		kind      string
		// wantBody is the body the push records, or empty for no entry.
		wantBody     string
		wantFromFile bool
		wantWarning  bool
	}{
		{
			name:          "a relative path with the placeholder",
			uri:           uri(fooPath),
			source:        fooPath,
			sourceContent: current,
			body:          Placeholder + "\n",
			wantBody:      current,
			wantFromFile:  true,
		},
		{
			// R-001: the agent copied the spec, then edited it three times
			// and did not touch the capture again.
			name:          "a stale copy is replaced",
			uri:           uri(fooPath),
			source:        fooPath,
			sourceContent: current,
			body:          "# Spec foo\n\nThe first draft.\n",
			wantBody:      current,
			wantFromFile:  true,
		},
		{
			name:          "no body at all",
			uri:           uri(fooPath),
			source:        fooPath,
			sourceContent: current,
			wantBody:      current,
			wantFromFile:  true,
		},
		{
			name:          "an absolute path",
			uri:           func(root, _ string) string { return filepath.Join(root, "notes", "design.md") },
			source:        "notes/design.md",
			sourceContent: current,
			body:          Placeholder,
			wantBody:      current,
			wantFromFile:  true,
		},
		{
			name:          "a path in the home directory",
			uri:           uri("~/notes/design.md"),
			source:        "home/notes/design.md",
			sourceContent: current,
			body:          Placeholder,
			wantBody:      current,
			wantFromFile:  true,
		},
		{
			name:          "a file URI",
			uri:           func(root, _ string) string { return "file://" + filepath.Join(root, "notes", "design.md") },
			source:        "notes/design.md",
			sourceContent: current,
			body:          Placeholder,
			wantBody:      current,
			wantFromFile:  true,
		},
		{
			// The rule applies to each kind whose capture file is text.
			name:          "a capture of kind text",
			uri:           uri("plan.md"),
			source:        "plan.md",
			sourceContent: current,
			kind:          aicodingsession.SpecKindText,
			body:          Placeholder,
			wantBody:      current,
			wantFromFile:  true,
		},
		{
			// R-003: the file was deleted before the push.
			name:        "a missing file with the placeholder",
			uri:         uri(gonePath),
			body:        Placeholder + "\n",
			wantWarning: true,
		},
		{
			name:        "a missing file with no body",
			uri:         uri(gonePath),
			wantWarning: true,
		},
		{
			name:     "a missing file with a body keeps the body",
			uri:      uri(gonePath),
			body:     agentBody,
			wantBody: agentBody,
		},
		{
			name:          "a file over the limit keeps the body",
			uri:           uri(bigPath),
			source:        bigPath,
			sourceContent: largeContent,
			body:          agentBody,
			wantBody:      agentBody,
		},
		{
			name:          "a file over the limit with the placeholder",
			uri:           uri(bigPath),
			source:        bigPath,
			sourceContent: largeContent,
			body:          Placeholder,
			wantWarning:   true,
		},
		{
			name:          "a file that is not text",
			uri:           uri("blob.bin"),
			source:        "blob.bin",
			sourceContent: binaryContent,
			body:          Placeholder,
			wantWarning:   true,
		},
		{
			// NUL is valid UTF-8, but no text file holds it.
			name:          "a valid UTF-8 file with a NUL byte",
			uri:           uri("blob.dat"),
			source:        "blob.dat",
			sourceContent: "header\x00binary payload",
			body:          Placeholder,
			wantWarning:   true,
		},
		{
			name:        "a directory is not a source",
			uri:         uri("docs"),
			source:      "docs",
			sourceDir:   true,
			body:        Placeholder,
			wantWarning: true,
		},
		{
			name:          "an empty file with the placeholder",
			uri:           uri("empty.md"),
			source:        "empty.md",
			sourceContent: "  \n",
			body:          Placeholder,
			wantWarning:   true,
		},
		{
			name:          "an empty file keeps the body",
			uri:           uri("empty.md"),
			source:        "empty.md",
			sourceContent: "  \n",
			body:          agentBody,
			wantBody:      agentBody,
		},
		{
			// A remote source is copied by the agent, as before.
			name:     "a URL is not a local file",
			uri:      uri(ticketURL),
			body:     "the ticket\n",
			wantBody: "the ticket\n",
		},
		{
			// The push never records the placeholder as content, whatever
			// the source.
			name:        "the placeholder for a URL",
			uri:         uri(ticketURL),
			body:        Placeholder,
			wantWarning: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "home")
			t.Setenv("HOME", home)

			if tc.source != "" {
				path := filepath.Join(root, tc.source)
				if tc.sourceDir {
					require.NoError(t, os.MkdirAll(path, 0o755))
				} else {
					writeSource(t, path, tc.sourceContent, fileTime)
				}
			}

			kind := tc.kind
			if kind == "" {
				kind = aicodingsession.SpecKindDocument
			}
			head := strings.Replace(header, "kind: document", "kind: "+kind, 1)
			head = strings.Replace(head, "%s", tc.uri(root, home), 1)
			writeSpec(t, root, sessionID, "spec-foo.md", head+tc.body, captureTime)

			entries, warnings, err := ReadAll(root, sessionID, nil)
			require.NoError(t, err, "a capture never fails the push")

			if tc.wantWarning {
				assert.Empty(t, entries, "the placeholder is never recorded as content")
				require.Len(t, warnings, 1)
				assert.Contains(t, warnings[0], "spec-foo.md", "the warning names the capture")
				assert.NotContains(t, warnings[0], root, "a warning never carries a local path")

				return
			}

			assert.Empty(t, warnings)
			require.Len(t, entries, 1)
			got := entries[0]

			assert.Equal(t, kind, got.Kind)
			assert.Equal(t, head+tc.wantBody, string(got.Raw), "the header the agent wrote stays, and the body is replaced")

			if tc.wantFromFile {
				assert.Equal(t, fileTime.Format(time.RFC3339), got.CapturedAt, "the capture time is the time of the file")
				assert.Equal(t, pointer.Digest([]byte(tc.sourceContent)), got.SourceDigest, "the content read is a source for the pointers")
			} else {
				assert.Equal(t, captureTime.Format(time.RFC3339), got.CapturedAt)
				assert.Empty(t, got.SourceDigest)
			}
		})
	}
}

// TestReadAllLocalSourceFollowsEdits checks the case of spec issue-3561: the
// agent captures a spec, edits it, and does not touch the capture again.
// Each push records the version on disk.
func TestReadAllLocalSourceFollowsEdits(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "docs", "specs", "foo.md")
	writeSpec(t, root, sessionID, "spec-foo.md", "---\nkind: document\nuri: docs/specs/foo.md\n---\n"+Placeholder+"\n", time.Now())

	for _, version := range []string{"first draft", "second draft", "third draft"} {
		writeSource(t, source, version, time.Now())

		entries, warnings, err := ReadAll(root, sessionID, nil)
		require.NoError(t, err)
		assert.Empty(t, warnings)
		require.Len(t, entries, 1)
		assert.True(t, strings.HasSuffix(string(entries[0].Raw), "---\n"+version), "the push records %q", version)
	}
}

func TestLocalPath(t *testing.T) {
	const (
		root = "/repo"
		home = "/home/dev"
		abs  = "/abs/notes/design.md"
	)

	testCases := []struct {
		uri    string
		want   string
		wantOK bool
	}{
		{uri: "docs/specs/foo.md", want: "/repo/docs/specs/foo.md", wantOK: true},
		{uri: "./docs/specs/foo.md", want: "/repo/docs/specs/foo.md", wantOK: true},
		{uri: abs, want: abs, wantOK: true},
		{uri: "~/notes/design.md", want: "/home/dev/notes/design.md", wantOK: true},
		{uri: "file://" + abs, want: abs, wantOK: true},
		{uri: "file:///abs/notes/my%20design.md", want: "/abs/notes/my design.md", wantOK: true},
		{uri: "file://localhost" + abs, want: abs, wantOK: true},
		{uri: "file://otherhost/abs/notes/design.md"},
		{uri: "file:relative.md"},
		{uri: "https://tracker.example.com/issue/1"},
		{uri: "http://example.com/doc"},
		{uri: "mailto:someone@example.com"},
		{uri: "urn:isbn:0451450523"},
		{uri: "~someone/notes.md"},
		{uri: ""},
		{uri: "   "},
	}

	for _, tc := range testCases {
		t.Run(tc.uri, func(t *testing.T) {
			got, ok := localPath(root, home, tc.uri)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
