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

package opencode

import (
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionSkills(t *testing.T) {
	const sessionID = "ses_skills"

	// skillPart is a skill tool call as OpenCode 1.x exports it. The tool
	// output holds the folder as a file URL, and the metadata as a path.
	skillPart := func(name, status, metadata, output string) string {
		return `{"type":"tool","tool":"skill","state":{"status":"` + status + `","input":{"name":"` + name + `"}` +
			metadata + `,"output":` + output + `}}`
	}
	message := func(created string, parts ...string) string {
		joined := ""
		for i, p := range parts {
			if i > 0 {
				joined += ","
			}
			joined += p
		}
		return `{"info":{"role":"assistant","id":"msg","time":{"created":` + created + `}},"parts":[` + joined + `]}`
	}
	export := func(messages ...string) string {
		joined := ""
		for i, m := range messages {
			if i > 0 {
				joined += ","
			}
			joined += m
		}
		return `{"info":{"id":"` + sessionID + `","directory":"/repo","time":{"created":1},"version":"1.18.0"},"messages":[` + joined + `]}`
	}

	testCases := []struct {
		name   string
		export string
		want   []trace.SkillUse
	}{
		{
			name:   "no skill",
			export: export(message("1791367200000", `{"type":"text","text":"hello"}`)),
			want:   []trace.SkillUse{},
		},
		{
			name: "completed calls count, a failed call does not",
			export: export(
				message("1791367200000", skillPart("git-release", "completed", `,"metadata":{"dir":"/home/me/.config/opencode/skills/git-release"}`, `"<skill_content name=\"git-release\">"`)),
				message("1791367260000", skillPart("missing", "error", "", `"Skill not found"`)),
				message("1791367320000", skillPart("git-release", "completed", "", `"Base directory for this skill: file:///home/me/.config/opencode/skills/git-release\n"`)),
				message("1791367380000", skillPart("pdf", "completed", "", `"Base directory for this skill: file:///repo/.opencode/skills/pdf\n"`)),
			),
			want: []trace.SkillUse{
				{Name: "git-release", Dir: "/home/me/.config/opencode/skills/git-release", FirstUsedAt: "2026-10-07T10:00:00Z", ByModel: 2},
				{Name: "pdf", Dir: "/repo/.opencode/skills/pdf", FirstUsedAt: "2026-10-07T10:03:00Z", ByModel: 1},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rawDir := t.TempDir()
			require.NoError(t, os.WriteFile(state.RawSessionPath(rawDir, sessionID), []byte(tc.export), 0o600))

			got, warnings, err := New().SessionSkills(&trace.ParseOpts{SessionDir: rawDir, SessionID: sessionID})
			require.NoError(t, err)
			assert.Empty(t, warnings)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNewSkillLoads(t *testing.T) {
	const pdfDir = "/skills/pdf"

	testCases := []struct {
		name  string
		input trace.HookInput
		want  []string
	}{
		{name: "a skill call with a folder", input: trace.HookInput{ToolName: "skill", SkillDir: pdfDir}, want: []string{pdfDir}},
		{name: "a skill call without a folder", input: trace.HookInput{ToolName: "skill"}},
		{name: "another tool", input: trace.HookInput{ToolName: "bash", SkillDir: pdfDir}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, New().NewSkillLoads(nil, &tc.input))
		})
	}
}
