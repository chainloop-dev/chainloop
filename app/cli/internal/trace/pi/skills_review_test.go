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

package pi

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/stretchr/testify/require"
)

func TestPiSessionSkillsModelReadLocations(t *testing.T) {
	// Given
	home := t.TempDir()
	repo := t.TempDir()
	cwd := filepath.Join(repo, "nested", "work")
	agentDir := filepath.Join(home, "custom-pi")
	require.NoError(t, os.MkdirAll(cwd, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(repo, ".git"), 0o755))
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)

	cases := []struct {
		name    string
		path    string
		tracked bool
	}{
		{name: "cwd pi skill", path: filepath.Join(cwd, ".pi", "skills", "review", "SKILL.md"), tracked: true},
		{name: "cwd agents skill", path: filepath.Join(cwd, ".agents", "skills", "review", "SKILL.md"), tracked: true},
		{name: "ancestor agents skill", path: filepath.Join(repo, ".agents", "skills", "review", "SKILL.md"), tracked: true},
		{name: "ancestor pi skill", path: filepath.Join(repo, ".pi", "skills", "review", "SKILL.md")},
		{name: "above repository", path: filepath.Join(filepath.Dir(repo), ".agents", "skills", "review", "SKILL.md")},
		{name: "agent skill", path: filepath.Join(agentDir, "skills", "review", "SKILL.md"), tracked: true},
		{name: "shared user skill", path: filepath.Join(home, ".agents", "skills", "review", "SKILL.md"), tracked: true},
		{name: "managed npm package", path: filepath.Join(agentDir, "npm", "node_modules", "@example", "tools", "custom", "SKILL.md"), tracked: true},
		{name: "managed git package", path: filepath.Join(agentDir, "git", "github.com", "example", "tools", "custom", "SKILL.md"), tracked: true},
		{name: "project npm package", path: filepath.Join(cwd, ".pi", "npm", "node_modules", "tools", "custom", "SKILL.md"), tracked: true},
		{name: "ordinary repository read", path: filepath.Join(repo, "docs", "review", "SKILL.md")},
		{name: "unconfigured claude skill", path: filepath.Join(repo, ".claude", "skills", "review", "SKILL.md")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawDir := t.TempDir()
			call := fmt.Sprintf(`{"role":"assistant","content":[{"type":"toolCall","id":"load-1","name":"read","arguments":{"path":%q}}]}`, tc.path)
			result := fmt.Sprintf(`{"role":"toolResult","toolCallId":"load-1","toolName":"read","content":[{"type":"text","text":%q}]}`, skillTestDefinition)
			writeRawSession(t, rawDir, piSkillMessages(cwd, []string{call, result}))

			// When
			uses, _, err := New().SessionSkills(&trace.ParseOpts{SessionID: testSessionID, SessionDir: rawDir})

			// Then
			require.NoError(t, err)
			if !tc.tracked {
				require.Empty(t, uses)
				return
			}
			require.Len(t, uses, 1)
			require.Equal(t, filepath.Dir(tc.path), uses[0].Dir)
		})
	}
}

func TestScanSkillTail(t *testing.T) {
	t.Run("unchanged transcript", func(t *testing.T) {
		// Given
		path := filepath.Join(t.TempDir(), "session.jsonl")
		data := []byte("header\n")
		require.NoError(t, os.WriteFile(path, data, 0o600))

		// When
		next, candidate, reset, err := scanSkillTail(path, int64(len(data)))

		// Then
		require.NoError(t, err)
		require.Equal(t, int64(len(data)), next)
		require.False(t, candidate)
		require.False(t, reset)
	})

	t.Run("irrelevant complete line", func(t *testing.T) {
		// Given
		path := filepath.Join(t.TempDir(), "session.jsonl")
		prefix := []byte("header\n")
		line := []byte(`{"type":"message","message":{"role":"assistant","content":"done"}}` + "\n")
		require.NoError(t, os.WriteFile(path, append(prefix, line...), 0o600))

		// When
		next, candidate, reset, err := scanSkillTail(path, int64(len(prefix)))

		// Then
		require.NoError(t, err)
		require.Equal(t, int64(len(prefix)+len(line)), next)
		require.False(t, candidate)
		require.False(t, reset)
	})

	t.Run("partial candidate waits for newline", func(t *testing.T) {
		// Given
		path := filepath.Join(t.TempDir(), "session.jsonl")
		prefix := []byte("header\n")
		partial := []byte(`{"message":{"role":"toolResult","toolName":"read"}}`)
		require.NoError(t, os.WriteFile(path, append(prefix, partial...), 0o600))

		// When
		next, candidate, reset, err := scanSkillTail(path, int64(len(prefix)))

		// Then
		require.NoError(t, err)
		require.Equal(t, int64(len(prefix)), next)
		require.False(t, candidate)
		require.False(t, reset)
	})

	t.Run("completed candidate triggers parse", func(t *testing.T) {
		// Given
		path := filepath.Join(t.TempDir(), "session.jsonl")
		prefix := []byte("header\n")
		line := []byte(`{"message":{"role":"toolResult","toolName":"read"}}` + "\n")
		require.NoError(t, os.WriteFile(path, append(prefix, line...), 0o600))

		// When
		next, candidate, reset, err := scanSkillTail(path, int64(len(prefix)))

		// Then
		require.NoError(t, err)
		require.Equal(t, int64(len(prefix)+len(line)), next)
		require.True(t, candidate)
		require.False(t, reset)
	})

	t.Run("shorter replacement resets cursor", func(t *testing.T) {
		// Given
		path := filepath.Join(t.TempDir(), "session.jsonl")
		data := []byte("header\n")
		require.NoError(t, os.WriteFile(path, data, 0o600))

		// When
		next, candidate, reset, err := scanSkillTail(path, int64(len(data)+10))

		// Then
		require.NoError(t, err)
		require.Equal(t, int64(len(data)), next)
		require.False(t, candidate)
		require.True(t, reset)
	})
}

func TestScanSkillTailCandidates(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		candidate bool
	}{
		{
			name: "bash output with unicode escape",
			line: `{"message":{"role":"toolResult","toolName":"bash","content":"\u001b[32mdone"}}`,
		},
		{
			name: "bash output with read substring",
			line: `{"message":{"role":"toolResult","toolName":"bash","content":"already on main"}}`,
		},
		{
			name: "bash output with ANSI escapes and read substring",
			line: `{"message":{"role":"toolResult","toolName":"bash","content":"\u001b[32malready on main\\^["}}`,
		},
		{
			name:      "native user skill expansion",
			line:      `{"message":{"role":"user","content":"<skill name=\"review\" location=\"skills/review/SKILL.md\">\nReview.\n</skill>"}}`,
			candidate: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			path := filepath.Join(t.TempDir(), "session.jsonl")
			data := []byte(tc.line + "\n")
			require.NoError(t, os.WriteFile(path, data, 0o600))

			// When
			next, candidate, reset, err := scanSkillTail(path, 0)

			// Then
			require.NoError(t, err)
			require.Equal(t, int64(len(data)), next)
			require.Equal(t, tc.candidate, candidate)
			require.False(t, reset)
		})
	}
}
