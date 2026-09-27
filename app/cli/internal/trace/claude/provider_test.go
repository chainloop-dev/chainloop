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

package claude

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranscriptSourceDir(t *testing.T) {
	const sid = "848a5f26-b5f8-4ef7-bf44-ea2d44beda3a"
	projects := filepath.Join(string(filepath.Separator), "home", "u", ".claude", "projects")
	projectDir := filepath.Join(projects, "-home-u-repo")

	cases := []struct {
		name           string
		transcriptPath string
		sessionID      string
		want           string
		wantErr        bool
	}{
		{
			name:           "main transcript",
			transcriptPath: filepath.Join(projectDir, sid+".jsonl"),
			sessionID:      sid,
			want:           projectDir,
		},
		{
			// In case a hook inside a subagent reports the subagent's own
			// transcript, the main one is two levels up.
			name:           "subagent transcript",
			transcriptPath: filepath.Join(projectDir, sid, "subagents", "agent-afd65659e2015d48d.jsonl"),
			sessionID:      sid,
			want:           projectDir,
		},
		{
			name:           "empty",
			transcriptPath: "",
			sessionID:      sid,
			wantErr:        true,
		},
		{
			name:           "relative path",
			transcriptPath: filepath.Join("-home-u-repo", sid+".jsonl"),
			sessionID:      sid,
			wantErr:        true,
		},
		{
			name:           "outside the projects directory",
			transcriptPath: filepath.Join(string(filepath.Separator), "tmp", sid+".jsonl"),
			sessionID:      sid,
			wantErr:        true,
		},
		{
			name:           "the projects directory itself",
			transcriptPath: filepath.Join(projects, sid+".jsonl"),
			sessionID:      sid,
			wantErr:        true,
		},
		{
			name:           "escapes the projects directory",
			transcriptPath: filepath.Join(projects, "..", "..", "evil", sid+".jsonl"),
			sessionID:      sid,
			wantErr:        true,
		},
		{
			name:           "another session's transcript",
			transcriptPath: filepath.Join(projectDir, "0000aaaa-b5f8-4ef7-bf44-ea2d44beda3a.jsonl"),
			sessionID:      sid,
			wantErr:        true,
		},
		{
			name:           "another session's subagent",
			transcriptPath: filepath.Join(projectDir, "0000aaaa-b5f8-4ef7-bf44-ea2d44beda3a", "subagents", "agent-1.jsonl"),
			sessionID:      sid,
			wantErr:        true,
		},
		{
			name:           "session ID with a path separator",
			transcriptPath: filepath.Join(projectDir, "x", "y.jsonl"),
			sessionID:      "x/y",
			wantErr:        true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := transcriptSourceDir(projects, tc.transcriptPath, tc.sessionID)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// The transcript directory Claude reports can differ from the one the hook's
// cwd encodes to, e.g. for a subagent working in its own git worktree. The
// reported path wins; the cwd encoding is only a fallback.
func TestCopySessionData(t *testing.T) {
	const sid = "848a5f26-b5f8-4ef7-bf44-ea2d44beda3a"

	cases := []struct {
		name string
		// transcriptPath returns the transcript_path the hook reports.
		transcriptPath func(reported string) string
		// wantFromReported is true when the copy comes from the reported
		// directory, false when it comes from the cwd encoding.
		wantFromReported bool
	}{
		{
			name:             "reported transcript path",
			transcriptPath:   func(reported string) string { return reported },
			wantFromReported: true,
		},
		{
			name:           "no transcript path falls back to cwd",
			transcriptPath: func(string) string { return "" },
		},
		{
			name:           "invalid transcript path falls back to cwd",
			transcriptPath: func(string) string { return filepath.Join(os.TempDir(), sid+".jsonl") },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			p := New()

			startDir := filepath.Join(home, "work", "repo")
			hookCwd := filepath.Join(startDir, ".claude", "worktrees", "agent-afd65659e2015d48d")

			// Claude files the transcript under the directory the session
			// started in, not under the directory the hook runs in.
			reportedDir := p.SessionDirForRepo(startDir)
			writeFile(t, filepath.Join(reportedDir, sid+".jsonl"), "reported-main\n")
			writeFile(t, filepath.Join(reportedDir, sid, "subagents", "agent-afd65659e2015d48d.jsonl"), "reported-sub\n")

			cwdDir := p.SessionDirForRepo(hookCwd)
			writeFile(t, filepath.Join(cwdDir, sid+".jsonl"), "cwd-main\n")

			store := state.NewGitStore(t.TempDir())
			err := p.CopySessionData(store, trace.SessionLocation{
				SessionID:      sid,
				Cwd:            hookCwd,
				TranscriptPath: tc.transcriptPath(filepath.Join(reportedDir, sid+".jsonl")),
			})
			require.NoError(t, err)

			rawDir := store.RawSessionDir()
			main, err := os.ReadFile(state.RawSessionPath(rawDir, sid))
			require.NoError(t, err)

			sub, subErr := os.ReadFile(filepath.Join(state.RawSubagentDir(rawDir, sid), "agent-afd65659e2015d48d.jsonl"))
			if tc.wantFromReported {
				assert.Equal(t, "reported-main\n", string(main))
				require.NoError(t, subErr, "subagent transcripts come along with the main one")
				assert.Equal(t, "reported-sub\n", string(sub))
			} else {
				assert.Equal(t, "cwd-main\n", string(main))
				assert.ErrorIs(t, subErr, os.ErrNotExist)
			}
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}
