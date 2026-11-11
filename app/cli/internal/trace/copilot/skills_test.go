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

//nolint:goconst // Keep native event literals readable in regression fixtures.
package copilot

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

const testSkillPath = "/work/repo/.github/skills/greeter/SKILL.md"

// skillInvoked is a skill.invoked event, as Copilot CLI 1.0.109 writes it.
func skillInvoked(t *testing.T, name, path, trigger, agentID, at string) string {
	t.Helper()
	event := map[string]any{
		"type":      eventSkillInvoked,
		"id":        "skill-" + name + "-" + at,
		"timestamp": at,
		"data": map[string]any{
			"name": name, "path": path, "trigger": trigger, "source": "project",
			"content": "Write the greeting file.\n",
		},
	}
	if agentID != "" {
		event["agentId"] = agentID
	}
	raw, err := json.Marshal(event)
	require.NoError(t, err)
	return string(raw)
}

func sessionSkills(t *testing.T, content string) ([]trace.SkillUse, error) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(state.RawSessionPath(dir, testSessionID), []byte(content), 0o600)) //nolint:gosec // temporary fixture path and a fixed session ID
	uses, _, err := New().SessionSkills(&trace.ParseOpts{SessionDir: dir, SessionID: testSessionID})
	return uses, err
}

func TestSessionSkills(t *testing.T) {
	const at1, at2 = "2026-04-07T21:00:05.120Z", "2026-04-07T21:00:09Z"
	// listing is the system message that lists the skills. It names the
	// skill and its file, but it is not a use.
	const listing = `{"type":"system.message","data":{"role":"system","content":"<available_skills><skill name=\"greeter\" path=\"` + testSkillPath + `\"/></available_skills>"},"id":"sys","timestamp":"2026-04-07T21:00:01Z"}`
	// failedTool is a skill tool call that found no skill: Copilot writes no
	// skill.invoked event for it.
	const failedTool = `{"type":"tool.execution_complete","data":{"toolCallId":"c1","success":false,"result":{"content":"Skill \"greeter\" not found"}},"id":"t","timestamp":"2026-04-07T21:00:02Z"}`

	tests := []struct {
		name    string
		lines   []string
		want    []trace.SkillUse
		wantErr string
	}{
		{
			name:  "model use",
			lines: []string{skillInvoked(t, "greeter", testSkillPath, "agent-invoked", "", at1)},
			want:  []trace.SkillUse{{Name: "greeter", Dir: filepath.Dir(testSkillPath), FirstUsedAt: "2026-04-07T21:00:05Z", ByModel: 1}},
		},
		{
			name:  "user use",
			lines: []string{skillInvoked(t, "greeter", testSkillPath, "user-invoked", "", at1)},
			want:  []trace.SkillUse{{Name: "greeter", Dir: filepath.Dir(testSkillPath), FirstUsedAt: "2026-04-07T21:00:05Z", ByUser: 1}},
		},
		{
			name: "repeated and subagent uses",
			lines: []string{
				skillInvoked(t, "greeter", testSkillPath, "agent-invoked", "", at2),
				skillInvoked(t, "greeter", testSkillPath, "agent-invoked", "agent-1", at1),
			},
			want: []trace.SkillUse{{Name: "greeter", Dir: filepath.Dir(testSkillPath), FirstUsedAt: "2026-04-07T21:00:05Z", ByModel: 2, InSubagents: 1}},
		},
		{
			name:  "listing and failed tool call are no use",
			lines: []string{listing, failedTool},
		},
		{
			name: "path that is not a skill definition",
			lines: []string{
				skillInvoked(t, "greeter", "/work/repo/.github/skills/greeter/README.md", "agent-invoked", "", at1),
				skillInvoked(t, "greeter", "relative/greeter/SKILL.md", "agent-invoked", "", at1),
				skillInvoked(t, "", testSkillPath, "agent-invoked", "", at1),
			},
		},
		{
			name:    "session mismatch",
			lines:   []string{skillInvoked(t, "greeter", testSkillPath, "agent-invoked", "", at1)},
			wantErr: "session ID mismatch",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first := header
			if tc.wantErr != "" {
				first = strings.Replace(header, testSessionID, "other-session", 1)
			}
			uses, err := sessionSkills(t, first+"\n"+strings.Join(tc.lines, "\n")+"\n")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			if tc.want == nil {
				assert.Empty(t, uses)
				return
			}
			assert.Equal(t, tc.want, uses)
		})
	}
}

func TestNewSkillLoads(t *testing.T) {
	use := skillInvoked(t, "greeter", testSkillPath, "agent-invoked", "", "2026-04-07T21:00:05Z")
	other := skillInvoked(t, "other", "/work/repo/.github/skills/other/SKILL.md", "agent-invoked", "", "2026-04-07T21:00:06Z")

	root := copilotHome(t, headerLine+use+"\n")
	logPath := filepath.Join(root, testSessionID, eventsFile)
	store := newStore(t)
	input := &trace.HookInput{SessionID: testSessionID, TranscriptPath: logPath}
	p := New()

	assert.Equal(t, []string{filepath.Dir(testSkillPath)}, p.NewSkillLoads(store, input))
	assert.Empty(t, p.NewSkillLoads(store, input), "a use is returned once")

	// A line that Copilot is still writing is read at the next hook.
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteString(other)
	require.NoError(t, err)
	assert.Empty(t, p.NewSkillLoads(store, input))
	_, err = f.WriteString("\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	assert.Equal(t, []string{"/work/repo/.github/skills/other"}, p.NewSkillLoads(store, input))

	// The computed path is used when the payload has none.
	fresh := newStore(t)
	assert.Equal(t, []string{filepath.Dir(testSkillPath), "/work/repo/.github/skills/other"},
		p.NewSkillLoads(fresh, &trace.HookInput{SessionID: testSessionID}))

	// A log outside the session-state folder is not read.
	outside := filepath.Join(t.TempDir(), eventsFile)
	require.NoError(t, os.WriteFile(outside, []byte(headerLine+use+"\n"), 0o600))
	assert.Empty(t, p.NewSkillLoads(newStore(t), &trace.HookInput{SessionID: testSessionID, TranscriptPath: outside}))

	assert.Empty(t, p.NewSkillLoads(nil, input))
	assert.Empty(t, p.NewSkillLoads(store, nil))
}

func TestSkillRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("COPILOT_HOME", home)
	t.Setenv("HOME", "/home/dev")

	roots := New().SkillRoots("/work/repo")
	assert.Equal(t, []string{"/work/repo"}, roots.Project)
	assert.Equal(t, []string{filepath.Join(home, "skills"), "/home/dev/.agents/skills"}, roots.User)
	assert.Equal(t, []string{filepath.Join(home, "installed-plugins")}, roots.Plugin)
}
