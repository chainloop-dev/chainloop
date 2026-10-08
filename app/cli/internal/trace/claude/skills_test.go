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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const skillsSessionID = "5c584dae-abf9-4a94-8598-4dc1ce6579d0"

const (
	steSkill   = "asd-ste100"
	graphSkill = "graphify"
	tenOClock  = "2026-10-07T10:00:00Z"
	tenOClockM = "2026-10-07T10:00:00.000Z"
	tenOhOne   = "2026-10-07T10:01:00Z"
)

// fixtureRecord, fixtureMessage and fixtureBlock have the shape that Claude
// Code 2.1 writes for a skill.
type fixtureRecord struct {
	Type            string         `json:"type"`
	UUID            string         `json:"uuid,omitempty"`
	ParentUUID      string         `json:"parentUuid,omitempty"`
	Timestamp       string         `json:"timestamp,omitempty"`
	IsMeta          bool           `json:"isMeta,omitempty"`
	SourceToolUseID string         `json:"sourceToolUseID,omitempty"`
	Message         fixtureMessage `json:"message"`
}

type fixtureMessage struct {
	Role string `json:"role"`
	// Content is a string or a list of blocks.
	Content any `json:"content"`
}

type fixtureBlock struct {
	Type      string            `json:"type"`
	Text      string            `json:"text,omitempty"`
	ID        string            `json:"id,omitempty"`
	Name      string            `json:"name,omitempty"`
	Input     map[string]string `json:"input,omitempty"`
	ToolUseID string            `json:"tool_use_id,omitempty"`
	Content   string            `json:"content,omitempty"`
	IsError   bool              `json:"is_error,omitempty"`
}

func toolCallLine(t *testing.T, uuid, toolID, skillName, ts string) string {
	t.Helper()

	return mustLine(t, fixtureRecord{
		Type: recordTypeAssistant, UUID: uuid, Timestamp: ts,
		Message: fixtureMessage{Role: recordTypeAssistant, Content: []fixtureBlock{{
			Type: "tool_use", ID: toolID, Name: skillTool,
			Input: map[string]string{"skill": skillName, "args": "private arguments"},
		}}},
	})
}

func toolResultLine(t *testing.T, uuid, parent, toolID, result string, isError bool, ts string) string {
	t.Helper()

	return mustLine(t, fixtureRecord{
		Type: recordTypeUser, UUID: uuid, ParentUUID: parent, Timestamp: ts,
		Message: fixtureMessage{Role: recordTypeUser, Content: []fixtureBlock{{
			Type: "tool_result", ToolUseID: toolID, Content: result, IsError: isError,
		}}},
	})
}

func commandLine(t *testing.T, uuid, command, ts string) string {
	t.Helper()

	return mustLine(t, fixtureRecord{
		Type: recordTypeUser, UUID: uuid, Timestamp: ts,
		Message: fixtureMessage{
			Role:    recordTypeUser,
			Content: "<command-message>" + command + "</command-message>\n<command-name>/" + command + "</command-name>",
		},
	})
}

// metaLine is the message that holds a loaded skill. toolID is empty for a
// skill that the user loaded, and dir for a skill with no folder.
func metaLine(t *testing.T, uuid, parent, toolID, dir, ts string) string {
	t.Helper()

	text := "Base directory for this skill: " + dir + "\n\n# The skill\n\nBody."
	if dir == "" {
		text = "`/simplify` Review the changed code.\n\nBody."
	}

	return mustLine(t, fixtureRecord{
		Type: recordTypeUser, UUID: uuid, ParentUUID: parent, Timestamp: ts, IsMeta: true, SourceToolUseID: toolID,
		Message: fixtureMessage{Role: recordTypeUser, Content: []fixtureBlock{{
			Type: "text", Text: text,
		}}},
	})
}

// mustLine encodes a record as Claude Code does, without escaping < and >.
func mustLine(t *testing.T, record fixtureRecord) string {
	t.Helper()

	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	require.NoError(t, enc.Encode(record))

	return strings.TrimSuffix(b.String(), "\n")
}

func writeLines(t *testing.T, path string, lines ...string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
}

func TestSessionSkills(t *testing.T) {
	const (
		steDir   = "/home/me/.claude/skills/asd-ste100"
		graphDir = "/home/me/.claude/skills/graphify"
		planDir  = "/home/me/.claude/plugins/cache/x/br/1.0.0/skills/plan"
	)

	modelStart := func(t *testing.T, n, skillName, dir, ts string) []string {
		return []string{
			toolCallLine(t, "a"+n, "toolu_"+n, skillName, ts),
			toolResultLine(t, "r"+n, "a"+n, "toolu_"+n, "Launching skill: "+skillName, false, ts),
			metaLine(t, "m"+n, "r"+n, "toolu_"+n, dir, ts),
		}
	}
	userStart := func(t *testing.T, n, command, dir, ts string) []string {
		return []string{
			commandLine(t, "c"+n, command, ts),
			metaLine(t, "m"+n, "c"+n, "", dir, ts),
		}
	}
	join := func(groups ...[]string) []string {
		var out []string
		for _, g := range groups {
			out = append(out, g...)
		}
		return out
	}

	testCases := []struct {
		name      string
		main      []string
		subagents map[string][]string
		want      []trace.SkillUse
	}{
		{
			name: "no skill",
			main: []string{commandLine(t, "c1", "clear", tenOClockM)},
			want: []trace.SkillUse{},
		},
		{
			name: "a model start",
			main: modelStart(t, "1", steSkill, steDir, "2026-10-07T10:14:51.622Z"),
			want: []trace.SkillUse{{Name: steSkill, Dir: steDir, FirstUsedAt: "2026-10-07T10:14:51Z", ByModel: 1}},
		},
		{
			name: "a user start",
			main: userStart(t, "1", graphSkill, graphDir, tenOClockM),
			want: []trace.SkillUse{{Name: graphSkill, Dir: graphDir, FirstUsedAt: tenOClock, ByUser: 1}},
		},
		{
			name: "both starts for one skill, in the order of first use",
			main: join(
				userStart(t, "1", graphSkill, graphDir, tenOClockM),
				modelStart(t, "2", steSkill, steDir, "2026-10-07T10:05:00.000Z"),
				modelStart(t, "3", graphSkill, graphDir, "2026-10-07T10:10:00.000Z"),
			),
			want: []trace.SkillUse{
				{Name: graphSkill, Dir: graphDir, FirstUsedAt: tenOClock, ByModel: 1, ByUser: 1},
				{Name: steSkill, Dir: steDir, FirstUsedAt: "2026-10-07T10:05:00Z", ByModel: 1},
			},
		},
		{
			name: "a failed call does not count",
			main: []string{
				toolCallLine(t, "a1", "toolu_1", "missing", tenOClockM),
				toolResultLine(t, "r1", "a1", "toolu_1", "<tool_use_error>Unknown skill: missing</tool_use_error>", true, tenOClockM),
			},
			want: []trace.SkillUse{},
		},
		{
			name: "built-in and custom commands do not count",
			main: []string{
				commandLine(t, "c1", "clear", tenOClockM),
				commandLine(t, "c2", "model", "2026-10-07T10:00:01.000Z"),
				commandLine(t, "c3", "my-custom-command", "2026-10-07T10:00:02.000Z"),
			},
			want: []trace.SkillUse{},
		},
		{
			name: "one use recorded twice counts once",
			main: join(
				modelStart(t, "1", steSkill, steDir, tenOClockM),
				[]string{metaLine(t, "m1-again", "r1", "toolu_1", steDir, tenOClockM)},
				userStart(t, "2", graphSkill, graphDir, "2026-10-07T10:01:00.000Z"),
				[]string{metaLine(t, "m2-again", "c2", "", graphDir, "2026-10-07T10:01:00.000Z")},
			),
			want: []trace.SkillUse{
				{Name: steSkill, Dir: steDir, FirstUsedAt: tenOClock, ByModel: 1},
				{Name: graphSkill, Dir: graphDir, FirstUsedAt: tenOhOne, ByUser: 1},
			},
		},
		{
			// A skill built into Claude Code, or a custom command, has no
			// folder. Its use is returned without one, and is not recorded.
			name: "a skill with no folder",
			main: join(
				userStart(t, "1", "simplify", "", tenOClockM),
				modelStart(t, "2", "code-review", "", "2026-10-07T10:01:00.000Z"),
			),
			want: []trace.SkillUse{
				{Name: "simplify", FirstUsedAt: tenOClock, ByUser: 1},
				{Name: "code-review", FirstUsedAt: tenOhOne, ByModel: 1},
			},
		},
		{
			name: "a plugin skill keeps its prefixed name",
			main: modelStart(t, "1", "br:plan", planDir, tenOClockM),
			want: []trace.SkillUse{{Name: "br:plan", Dir: planDir, FirstUsedAt: tenOClock, ByModel: 1}},
		},
		{
			name: "subagent uses merge into the session entry",
			main: modelStart(t, "1", steSkill, steDir, "2026-10-07T10:05:00.000Z"),
			subagents: map[string][]string{
				"agent-a1.jsonl": modelStart(t, "s1", steSkill, steDir, "2026-10-07T10:01:00.000Z"),
				"agent-a2.jsonl": modelStart(t, "s2", graphSkill, graphDir, "2026-10-07T10:09:00.000Z"),
			},
			want: []trace.SkillUse{
				{Name: steSkill, Dir: steDir, FirstUsedAt: tenOhOne, ByModel: 2, InSubagents: 1},
				{Name: graphSkill, Dir: graphDir, FirstUsedAt: "2026-10-07T10:09:00Z", ByModel: 1, InSubagents: 1},
			},
		},
		{
			// A format that this version does not know gives no entry, never
			// a failed push.
			name: "an unknown format gives no entry",
			main: []string{mustLine(t, fixtureRecord{
				Type: recordTypeUser, UUID: "x", Message: fixtureMessage{Role: recordTypeUser, Content: "Launching skill: " + steSkill},
			})},
			want: []trace.SkillUse{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rawDir := t.TempDir()
			writeLines(t, state.RawSessionPath(rawDir, skillsSessionID), tc.main...)
			for name, lines := range tc.subagents {
				writeLines(t, filepath.Join(state.RawSubagentDir(rawDir, skillsSessionID), name), lines...)
			}

			opts := &trace.ParseOpts{SessionDir: rawDir, SessionID: skillsSessionID}
			got, warnings, err := New().SessionSkills(opts)
			require.NoError(t, err)
			assert.Empty(t, warnings)
			assert.Equal(t, tc.want, got)

			again, _, err := New().SessionSkills(opts)
			require.NoError(t, err)
			assert.Equal(t, got, again, "the same transcript gives the same output")
		})
	}
}

func TestNewSkillLoads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectDir := filepath.Join(home, ".claude", "projects", "-repo")
	transcript := filepath.Join(projectDir, skillsSessionID+".jsonl")
	subagent := filepath.Join(projectDir, skillsSessionID, "subagents", "agent-a1.jsonl")

	store := state.NewGitStore(filepath.Join(t.TempDir(), ".git"))
	input := &trace.HookInput{SessionID: skillsSessionID, TranscriptPath: transcript}

	writeLines(t, transcript,
		commandLine(t, "c1", graphSkill, tenOClockM),
		metaLine(t, "m1", "c1", "", "/skills/graphify", tenOClockM),
	)

	assert.Equal(t, []string{"/skills/graphify"}, New().NewSkillLoads(store, input))
	assert.Empty(t, New().NewSkillLoads(store, input), "a line is read once")

	// The agent appends a use, and writes half of the next line.
	f, err := os.OpenFile(transcript, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString(metaLine(t, "m2", "r2", "toolu_2", "/skills/asd-ste100", "2026-10-07T10:01:00.000Z") + "\n")
	require.NoError(t, err)
	half := metaLine(t, "m3", "r3", "toolu_3", "/skills/later", "2026-10-07T10:02:00.000Z")
	_, err = f.WriteString(half[:len(half)/2])
	require.NoError(t, err)
	require.NoError(t, f.Close())

	writeLines(t, subagent, metaLine(t, "s1", "rs1", "toolu_s1", "/skills/in-subagent", "2026-10-07T10:01:30.000Z"))

	assert.ElementsMatch(t, []string{"/skills/asd-ste100", "/skills/in-subagent"}, New().NewSkillLoads(store, input),
		"a line that the agent is still writing waits for the next hook")

	f, err = os.OpenFile(transcript, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString(half[len(half)/2:] + "\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	assert.Equal(t, []string{"/skills/later"}, New().NewSkillLoads(store, input))
}
