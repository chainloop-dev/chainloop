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

package action

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/skill"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// steLintScript is the script of the STE test skill.
const steLintScript = "scripts/ste-lint.py"

// transcript appends Claude Code transcript records to the session file, in
// the shape that Claude Code 2.1 writes them.
type transcript struct {
	t    *testing.T
	path string
}

func (tr transcript) append(records ...string) {
	tr.t.Helper()

	f, err := os.OpenFile(tr.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(tr.t, err)
	defer func() { _ = f.Close() }()

	for _, r := range records {
		_, err := f.WriteString(r + "\n")
		require.NoError(tr.t, err)
	}
}

// TestSkillScenario follows two skills from their use in a Claude Code session
// to the materials that the pre-push hook adds to the attestation. The model
// starts one skill with the Skill tool, and the user starts the other with a
// slash command. The hooks run as Claude Code runs them, with the payload on
// stdin, and the push runs every step up to the upload.
func TestSkillScenario(t *testing.T) {
	const (
		sessionID = "0f3a6c1e-2b4d-4e8f-9a1b-2c3d4e5f6a7b"
		// Assembled from fragments so that nothing credential-shaped is
		// committed.
		pat = "ghp_erOZlZv0B1e3amrQ" + "ugdwZ8Ro2W4kDql9WPTf"
	)

	home := t.TempDir()
	t.Setenv("HOME", home)

	repoDir := initTempGitRepo(t)
	require.NoError(t, state.NewGitStore(filepath.Join(repoDir, ".git")).InitTraceDir())
	t.Chdir(repoDir)

	// A user skill with a script and a version-control folder, and a project
	// skill in the repository.
	userSkill := makeSkillAt(t, filepath.Join(home, ".claude", "skills", "asd-ste100"), map[string]string{
		skill.DefinitionFile: "# STE\nAs it ran. Token " + pat + "\n",
		steLintScript:        "TOKEN = '" + pat + "'\n",
		".git/HEAD":          "ref: refs/heads/main\n",
	})
	projectSkill := makeSkillAt(t, filepath.Join(repoDir, ".claude", "skills", "release"), map[string]string{
		skill.DefinitionFile: "# Release\nCut a release.\n",
	})

	projectDir := claudeProjectDir(home, repoDir)
	require.NoError(t, os.MkdirAll(projectDir, 0o700))
	tr := transcript{t: t, path: filepath.Join(projectDir, sessionID+".jsonl")}

	quote := func(s string) string {
		b, err := json.Marshal(s)
		require.NoError(t, err)
		return string(b)
	}

	// hookPayload is what Claude Code writes to the stdin of a hook.
	hookPayload := func(event, tool string) string {
		b, err := json.Marshal(struct {
			SessionID      string `json:"session_id"`
			HookEventName  string `json:"hook_event_name"`
			TranscriptPath string `json:"transcript_path"`
			Cwd            string `json:"cwd"`
			ToolName       string `json:"tool_name,omitempty"`
		}{sessionID, event, tr.path, repoDir, tool})
		require.NoError(t, err)
		return string(b)
	}

	provider := claude.New()

	// The session starts.
	tr.append(`{"type":"user","uuid":"u1","timestamp":"2026-10-07T10:00:00.000Z","sessionId":"` + sessionID + `","message":{"role":"user","content":"Rewrite the PR description in STE."}}`)
	withStdin(t, hookPayload("SessionStart", ""))
	captureStdout(t, func() { require.NoError(t, HandleAgentSessionStart(provider, zerolog.Nop())) })

	// The model calls the Skill tool, Claude Code loads the skill, and the
	// post-tool-use hook fires.
	tr.append(
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","timestamp":"2026-10-07T10:01:00.000Z","message":{"model":"claude-opus-5-5","role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Skill","input":{"skill":"asd-ste100","args":"the PR description"}}],"usage":{"input_tokens":10,"output_tokens":5}}}`,
		`{"type":"user","uuid":"r1","parentUuid":"a1","timestamp":"2026-10-07T10:01:01.000Z","toolUseResult":{"success":true,"commandName":"asd-ste100"},"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"Launching skill: asd-ste100"}]}}`,
		`{"type":"user","uuid":"m1","parentUuid":"r1","isMeta":true,"sourceToolUseID":"toolu_1","timestamp":"2026-10-07T10:01:01.000Z","message":{"role":"user","content":[{"type":"text","text":`+quote("Base directory for this skill: "+userSkill+"\n\n# STE")+`}]}}`,
	)
	withStdin(t, hookPayload("PostToolUse", "Skill"))
	require.NoError(t, HandleAgentPostToolUse(provider, zerolog.Nop()))

	// The skill changes on disk after it ran. The evidence must hold the
	// skill as it ran.
	require.NoError(t, os.WriteFile(filepath.Join(userSkill, skill.DefinitionFile), []byte("# STE\nEdited after the use.\n"), 0o600))

	// The user starts the project skill with a slash command, and the next
	// prompt fires the prompt-submit hook.
	tr.append(
		`{"type":"user","uuid":"c1","timestamp":"2026-10-07T10:05:00.000Z","message":{"role":"user","content":"<command-message>release</command-message>\n<command-name>/release</command-name>"}}`,
		`{"type":"user","uuid":"m2","parentUuid":"c1","isMeta":true,"timestamp":"2026-10-07T10:05:00.000Z","message":{"role":"user","content":[{"type":"text","text":`+quote("Base directory for this skill: "+projectSkill+"\n\n# Release")+`}]}}`,
		`{"type":"user","uuid":"u2","timestamp":"2026-10-07T10:06:00.000Z","message":{"role":"user","content":"thanks"}}`,
	)
	withStdin(t, hookPayload("UserPromptSubmit", ""))
	captureStdout(t, func() { require.NoError(t, HandleAgentPromptSubmit(provider, zerolog.Nop())) })

	// The pre-push hook: the push builds the session evidence and adds the
	// materials, as RunTracePush does once the attestation exists.
	ctx := context.Background()
	store, repoRoot, err := state.Locate()
	require.NoError(t, err)
	records, err := store.LoadAllSessionRecords()
	require.NoError(t, err)

	sessions := buildSessionEvidence(ctx, store, repoRoot, tracegit.NewGoGitClient(),
		map[string][]*state.CommitRecord{sessionID: nil}, records, aicodingsession.ModeCoding, zerolog.Nop())
	require.Len(t, sessions, 1)

	adder := &fakeMaterialAdder{realDigests: true}
	attested, _ := attachSessionEvidence(ctx, adder, store, sessions, zerolog.Nop(), false)
	require.Equal(t, []string{sessionID}, attested)

	// Each skill gives its definition and its package, then the session
	// material comes last.
	assert.Equal(t, []string{
		"spec-0f3a6c-skill-asd-ste100",
		"spec-0f3a6c-skill-asd-ste100-pkg",
		"spec-0f3a6c-skill-release",
		"spec-0f3a6c-skill-release-pkg",
		"ai-coding-session-0f3a6c",
	}, adder.names())

	definition := adder.byName("spec-0f3a6c-skill-asd-ste100")
	assert.Equal(t, "EVIDENCE", definition.kind)
	assert.Equal(t, skill.DefinitionFile, definition.fileName)
	assert.Contains(t, definition.content, "As it ran.", "the evidence holds the skill as it ran")
	assert.NotContains(t, definition.content, pat, "the definition is redacted")
	assert.Equal(t, skillContentDefinition, definition.annotations[specAnnotationSkillContent])

	pkg := adder.byName("spec-0f3a6c-skill-asd-ste100-pkg")
	assert.Equal(t, "EVIDENCE", pkg.kind)
	assert.Equal(t, "asd-ste100.tar.gz", pkg.fileName)
	assert.Equal(t, skillContentPackage, pkg.annotations[specAnnotationSkillContent])
	files := untar(t, pkg.content)
	assert.ElementsMatch(t, []string{skill.DefinitionFile, steLintScript}, slices.Collect(maps.Keys(files)), "the package holds the full folder, without .git")
	assert.Equal(t, definition.content, files[skill.DefinitionFile])
	assert.NotContains(t, files[steLintScript], pat, "each file of the package is redacted")

	// The session material holds the skill entries, which point to the
	// materials above, and passes the schema.
	session := adder.byName("ai-coding-session-0f3a6c")
	assert.Equal(t, "CHAINLOOP_AI_CODING_SESSION", session.kind)

	var evidence aicodingsession.Evidence
	require.NoError(t, json.Unmarshal([]byte(session.content), &evidence))
	require.Len(t, evidence.Data.Spec, 2)

	digestOf := func(name string) string { return adder.byName(name).digest }

	assert.Equal(t, aicodingsession.SpecEntry{
		Kind: aicodingsession.SpecKindSkill, Title: "asd-ste100",
		Digest: digestOf("spec-0f3a6c-skill-asd-ste100"), CapturedAt: "2026-10-07T10:01:00Z",
		Metadata: &aicodingsession.SpecMetadata{
			Source: aicodingsession.SkillSourceUser, PackageDigest: digestOf("spec-0f3a6c-skill-asd-ste100-pkg"),
			InvocationCount: 1, ByModel: 1,
		},
	}, evidence.Data.Spec[0])
	assert.Equal(t, aicodingsession.SpecEntry{
		Kind: aicodingsession.SpecKindSkill, Title: "release",
		Digest: digestOf("spec-0f3a6c-skill-release"), CapturedAt: "2026-10-07T10:05:00Z",
		Metadata: &aicodingsession.SpecMetadata{
			Source: aicodingsession.SkillSourceProject, PackageDigest: digestOf("spec-0f3a6c-skill-release-pkg"),
			InvocationCount: 1, ByUser: 1,
		},
	}, evidence.Data.Spec[1])

	for _, w := range evidence.Data.Warnings {
		assert.NotContains(t, w, "skill", "the hooks copied both skills at their use")
	}

	// The material crafter validates the data of the envelope, decoded as
	// generic JSON so that an unknown field fails the schema.
	var envelope struct {
		Data any `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(session.content), &envelope))
	require.NoError(t, schemavalidators.ValidateAICodingSession(envelope.Data, schemavalidators.AICodingSessionVersion0_1))
}
