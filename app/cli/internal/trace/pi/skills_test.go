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
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/skill"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/require"
)

const (
	skillTestDefinition = "---\nname: review\ndescription: Review the implementation.\n---\nRead the surrounding code.\n"
	skillSnapshotTool   = "write"
)

func piSkillMessages(cwd string, messages []string) []byte {
	lines := make([]string, 0, 1+len(messages))
	lines = append(lines, fmt.Sprintf(`{"type":"session","version":3,"id":%q,"cwd":%q,"timestamp":"2026-10-30T10:00:00Z"}`, testSessionID, cwd))
	for i, message := range messages {
		parent := "null"
		if i > 0 {
			parent = fmt.Sprintf("%q", fmt.Sprintf("entry-%d", i-1))
		}
		lines = append(lines, fmt.Sprintf(`{"type":"message","id":"entry-%d","parentId":%s,"timestamp":"2026-10-30T10:00:%02dZ","message":%s}`,
			i, parent, i+1, message))
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func TestPiSessionSkills(t *testing.T) {
	// Given
	tracker, ok := any(New()).(trace.SkillTracker)
	require.True(t, ok, "Pi must implement the shared skill tracker")
	read := `{"role":"assistant","content":[{"type":"toolCall","id":"load-1","name":"read","arguments":{"path":"skills/review/SKILL.md"}}]}`
	result := fmt.Sprintf(`{"role":"toolResult","toolCallId":"load-1","toolName":"read","isError":false,"content":[{"type":"text","text":%q}]}`, skillTestDefinition)
	cases := []struct {
		name     string
		messages []string
		model    int
		user     int
	}{
		{name: "no loads", messages: []string{`{"role":"user","content":"List available skills."}`}},
		{name: "available catalog is not a use", messages: []string{`{"role":"user","content":"<available_skills><skill><name>review</name><location>/skills/review/SKILL.md</location></skill></available_skills>"}`}},
		{name: "completed read", messages: []string{read, result}, model: 1},
		{name: "CRLF definition", messages: []string{read, strings.Replace(result, fmt.Sprintf("%q", skillTestDefinition), fmt.Sprintf("%q", strings.ReplaceAll(skillTestDefinition, "\n", "\r\n")), 1)}, model: 1},
		{name: "unfinished read", messages: []string{read}},
		{name: "failed read", messages: []string{read, strings.Replace(result, `"isError":false`, `"isError":true`, 1)}},
		{name: "partial read", messages: []string{strings.Replace(read, `"path":`, `"offset":2,"path":`, 1), result}},
		{name: "failed assistant response", messages: []string{strings.Replace(read, `"role":"assistant"`, `"role":"assistant","stopReason":"error"`, 1)}},
		{name: "result without matching call", messages: []string{result}},
		{name: "non-skill read", messages: []string{strings.Replace(read, "SKILL.md", "README.md", 1), result}},
		{name: "result without a definition", messages: []string{read, strings.Replace(result, fmt.Sprintf("%q", skillTestDefinition), `"unrelated output"`, 1)}},
		{name: "duplicate result counts once", messages: []string{read, result, result}, model: 1},
		{name: "repeated completed loads", messages: []string{read, result, strings.ReplaceAll(read, "load-1", "load-2"), strings.ReplaceAll(result, "load-1", "load-2")}, model: 2},
		{name: "user skill expansion", messages: []string{`{"role":"user","content":"<skill name=\"review\" location=\"skills/review/SKILL.md\">\nReferences are relative to skills/review.\n\nReview the code.\n</skill>"}`}, user: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			rawDir := t.TempDir()
			writeRawSession(t, rawDir, piSkillMessages(root, tc.messages))

			// When
			uses, warnings, err := tracker.SessionSkills(&trace.ParseOpts{SessionID: testSessionID, SessionDir: rawDir})

			// Then
			require.NoError(t, err)
			require.Empty(t, warnings)
			if tc.model+tc.user == 0 {
				require.Empty(t, uses)
				return
			}
			require.Len(t, uses, 1)
			require.Equal(t, "review", uses[0].Name)
			require.Equal(t, filepath.Join(root, "skills", "review"), uses[0].Dir)
			require.Equal(t, tc.model, uses[0].ByModel)
			require.Equal(t, tc.user, uses[0].ByUser)
			require.Zero(t, uses[0].InSubagents)
		})
	}
}

func TestPiSkillRoots(t *testing.T) {
	// Given
	home := t.TempDir()
	repo := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, "custom-pi"))
	roots := New().SkillRoots(repo)
	cases := []struct {
		path   string
		source string
	}{
		{filepath.Join(repo, ".agents", "skills", "review"), "project"},
		{filepath.Join(repo, ".pi", "skills", "review"), "project"},
		{filepath.Join(home, "custom-pi", "skills", "review"), "user"},
		{filepath.Join(home, ".agents", "skills", "review"), "user"},
		{filepath.Join(home, "custom-pi", "npm", "node_modules", "example", "skills", "review"), "plugin"},
		{filepath.Join(repo, ".pi", "npm", "node_modules", "example", "skills", "review"), "plugin"},
		{filepath.Join(t.TempDir(), "organization", "review"), "unknown"},
	}
	for _, tc := range cases {
		// When
		source := skill.Source("review", tc.path, roots)

		// Then
		require.Equal(t, tc.source, source)
	}
}

func TestPiSessionSkillsSelectedBranchAndFirstUseOrder(t *testing.T) {
	// Given
	tracker, ok := any(New()).(trace.SkillTracker)
	require.True(t, ok)
	root := t.TempDir()
	rawDir := t.TempDir()
	messages := []string{
		`{"role":"user","content":"<skill name=\"review\" location=\"skills/review/SKILL.md\">\nReview.\n</skill>"}`,
		`{"role":"user","content":[{"type":"text","text":"<skill name=\"build\" location=\"skills/build/SKILL.md\">\nBuild.\n</skill>"}]}`,
		`{"role":"user","content":"<skill name=\"review\" location=\"skills/review/SKILL.md\">\nReview again.\n</skill>"}`,
	}
	transcript := piSkillMessages(root, messages)
	writeRawSession(t, rawDir, transcript)

	// When
	uses, _, err := tracker.SessionSkills(&trace.ParseOpts{SessionID: testSessionID, SessionDir: rawDir})

	// Then
	require.NoError(t, err)
	require.Len(t, uses, 2)
	require.Equal(t, "review", uses[0].Name)
	require.Equal(t, 2, uses[0].ByUser)
	require.Equal(t, "2026-10-30T10:00:01Z", uses[0].FirstUsedAt)
	require.Equal(t, "build", uses[1].Name)

	t.Run("abandoned branch is excluded", func(t *testing.T) {
		alternate := `{"type":"message","id":"alternate","parentId":null,"timestamp":"2026-10-30T10:01:00Z","message":{"role":"user","content":"No skills on this branch."}}` + "\n"
		writeRawSession(t, rawDir, append(transcript, []byte(alternate)...))
		selected, _, err := tracker.SessionSkills(&trace.ParseOpts{SessionID: testSessionID, SessionDir: rawDir})
		require.NoError(t, err)
		require.Empty(t, selected)
	})
}

func TestPiNewSkillLoadsWaitsForCompletedResult(t *testing.T) {
	// Given
	tracker, ok := any(New()).(trace.SkillTracker)
	require.True(t, ok)
	root := t.TempDir()
	store := state.NewGitStore(filepath.Join(root, ".git"))
	require.NoError(t, store.InitTraceDir())
	transcript := filepath.Join(root, "live.jsonl")
	input := &trace.HookInput{SessionID: testSessionID, TranscriptPath: transcript, ToolName: skillSnapshotTool}
	call := `{"role":"assistant","content":[{"type":"toolCall","id":"load-1","name":"read","arguments":{"path":"skills/review/SKILL.md"}}]}`
	result := fmt.Sprintf(`{"role":"toolResult","toolCallId":"load-1","toolName":"read","content":[{"type":"text","text":%q}]}`, skillTestDefinition)
	require.NoError(t, os.WriteFile(transcript, piSkillMessages(root, []string{call}), 0o600))
	require.Empty(t, tracker.NewSkillLoads(store, input))

	// When
	require.NoError(t, os.WriteFile(transcript, piSkillMessages(root, []string{call, result}), 0o600))
	loads := tracker.NewSkillLoads(store, input)

	// Then
	require.Equal(t, []string{filepath.Join(root, "skills", "review")}, loads)
	require.Empty(t, tracker.NewSkillLoads(store, input), "an unchanged transcript is not a new use")
	input.SessionID = "another-session"
	require.Empty(t, tracker.NewSkillLoads(store, input), "a different session cannot supply skill snapshots")
}
