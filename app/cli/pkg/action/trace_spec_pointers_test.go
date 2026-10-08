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
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/cursor"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mainStream is the key of the main conversation in a raw session.
const mainStream = "main"

// TestAttachSessionEvidenceSpecPointers follows R-001: the agent reads a
// screenshot and copies it into the spec folder, and writes a ticket file
// into the spec folder. The session material holds a pointer for each copy
// of both, and no inline content of them.
func TestAttachSessionEvidenceSpecPointers(t *testing.T) {
	const sessionID = "7c1e2d-session"

	screenshotBytes := []byte("\x89PNG\r\n\x1a\nscreenshot of the bug")
	screenshotPath := filepath.Join(t.TempDir(), "screenshot.png")
	require.NoError(t, os.WriteFile(screenshotPath, screenshotBytes, 0o600))
	b64 := base64.StdEncoding.EncodeToString(screenshotBytes)

	// The ticket holds a secret, so its material is the redacted copy and its
	// digest differs from the digest of the file the agent wrote.
	const pat = "ghp_erOZlZv0B1e3amrQ" + "ugdwZ8Ro2W4kDql9WPTf"
	const ticketText = "---\nkind: ticket\nuri: https://tracker.example.com/ENG-1234\ntitle: Add an export button\n---\n\nThe export button is missing.\nToken: " + pat + "\n"
	ticketPath := "/repo/.chainloop/specs/" + sessionID + "/ticket-eng-1234.md"

	raw := []json.RawMessage{
		json.RawMessage(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_r","name":"Read","input":{"file_path":` + strconv.Quote(screenshotPath) + `}}]}}`),
		json.RawMessage(`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_r","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"` + b64 + `"}}]}]},"toolUseResult":{"type":"image","file":{"base64":"` + b64 + `","type":"image/png","originalSize":` + strconv.Itoa(len(screenshotBytes)) + `}}}`),
		json.RawMessage(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_w","name":"Write","input":{"file_path":` + strconv.Quote(ticketPath) + `,"content":` + strconv.Quote(ticketText) + `}}]}}`),
		json.RawMessage(`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_w","content":"File created successfully"}]},"toolUseResult":{"type":"create","filePath":` + strconv.Quote(ticketPath) + `,"content":` + strconv.Quote(ticketText) + `,"structuredPatch":[],"originalFile":null}}`),
	}

	ticket := specCapture(t, "ticket-eng-1234.md", ticketText, "2026-10-08T10:00:00Z")
	screenshot := spec.Capture{
		FileName: "screenshot.png", Kind: aicodingsession.SpecKindImage,
		CapturedAt: "2026-10-08T10:00:00Z", Raw: screenshotBytes, Verbatim: true,
	}

	evidence := aicodingsession.NewEvidence(aicodingsession.Data{
		SchemaVersion: "0.1",
		RawSession:    map[string][]json.RawMessage{mainStream: raw},
	})

	adder := &fakeMaterialAdder{realDigests: true}
	attested, _ := attachSessionEvidence(context.Background(), adder, state.NewGitStore(t.TempDir()), []sessionEvidence{{
		sessionID: sessionID,
		provider:  claude.New(),
		evidence:  evidence,
		specs:     []spec.Capture{ticket, screenshot},
	}}, zerolog.Nop())
	require.Equal(t, []string{sessionID}, attested)

	ticketDigest := adder.byName("spec-7c1e2d-ticket-eng-1234").digest
	screenshotDigest := adder.byName("spec-7c1e2d-screenshot").digest
	require.NotEmpty(t, ticketDigest)
	require.NotEmpty(t, screenshotDigest)

	session := adder.byName("ai-coding-session-7c1e2d")
	assert.NotContains(t, session.content, b64, "no inline copy of the screenshot")
	assert.NotContains(t, session.content, "The export button is missing", "no inline copy of the ticket")
	assert.NotContains(t, adder.byName("spec-7c1e2d-ticket-eng-1234").content, pat, "the ticket material is the redacted copy")

	var got aicodingsession.Evidence
	require.NoError(t, json.Unmarshal([]byte(session.content), &got))

	// Each pointer holds the digest of a material that the spec list names.
	specDigests := map[string]bool{}
	for _, e := range got.Data.Spec {
		specDigests[e.Digest] = true
	}
	assert.True(t, specDigests[ticketDigest])
	assert.True(t, specDigests[screenshotDigest])

	type ptr struct {
		Type   string `json:"type"`
		Digest string `json:"digest"`
	}
	var result struct {
		Message struct {
			Content []struct {
				Content []struct {
					Type   string `json:"type"`
					Source ptr    `json:"source"`
				} `json:"content"`
			} `json:"content"`
		} `json:"message"`
		ToolUseResult struct {
			File map[string]json.RawMessage `json:"file"`
		} `json:"toolUseResult"`
	}
	lines := got.Data.RawSession[mainStream]
	require.NoError(t, json.Unmarshal(lines[1], &result))
	assert.Equal(t, "image", result.Message.Content[0].Content[0].Type)
	assert.Equal(t, ptr{Type: pointer.Marker, Digest: screenshotDigest}, result.Message.Content[0].Content[0].Source)
	assert.JSONEq(t, `{"type":"chainloop.replaced","digest":"`+screenshotDigest+`"}`, string(result.ToolUseResult.File[pointer.Marker]))

	var write struct {
		Message struct {
			Content []struct {
				Input map[string]json.RawMessage `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(lines[2], &write))
	assert.JSONEq(t, `{"type":"chainloop.replaced","digest":"`+ticketDigest+`"}`, string(write.Message.Content[0].Input[pointer.Marker]))

	var writeResult struct {
		ToolUseResult map[string]json.RawMessage `json:"toolUseResult"`
	}
	require.NoError(t, json.Unmarshal(lines[3], &writeResult))
	assert.JSONEq(t, `{"type":"chainloop.replaced","digest":"`+ticketDigest+`"}`, string(writeResult.ToolUseResult[pointer.Marker]))
}

// TestAttachSessionEvidenceLocalSourcePointers follows R-005 of spec
// issue-3561: the agent writes a design note in one full write, reads it in
// full, and captures it by its path with the placeholder. The push records
// the note, and both copies in the transcript become pointers to it.
func TestAttachSessionEvidenceLocalSourcePointers(t *testing.T) {
	const sessionID = "9a8b7c-session"
	const note = "# Design note\n\nThe export runs in the background.\n"

	repoRoot := t.TempDir()
	notePath := filepath.Join(t.TempDir(), "notes", "design.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(notePath), 0o755))
	require.NoError(t, os.WriteFile(notePath, []byte(note), 0o600))

	require.NoError(t, os.MkdirAll(spec.SessionDir(repoRoot, sessionID), 0o755))
	capture := "---\nkind: document\nuri: " + notePath + "\nrole: spec\ntitle: Design note\n---\n" + spec.Placeholder + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(spec.SessionDir(repoRoot, sessionID), "design-note.md"), []byte(capture), 0o600))

	captures, warnings, err := spec.ReadAll(repoRoot, sessionID, nil)
	require.NoError(t, err)
	require.Empty(t, warnings)
	require.Len(t, captures, 1)

	numLines := strconv.Itoa(strings.Count(note, "\n"))
	raw := []json.RawMessage{
		json.RawMessage(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_w","name":"Write","input":{"file_path":` + strconv.Quote(notePath) + `,"content":` + strconv.Quote(note) + `}}]}}`),
		json.RawMessage(`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_w","content":"File created successfully"}]},"toolUseResult":{"type":"create","filePath":` + strconv.Quote(notePath) + `,"content":` + strconv.Quote(note) + `,"structuredPatch":[],"originalFile":null}}`),
		json.RawMessage(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_r","name":"Read","input":{"file_path":` + strconv.Quote(notePath) + `}}]}}`),
		json.RawMessage(`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_r","content":` + strconv.Quote(note) + `}]},"toolUseResult":{"type":"text","file":{"filePath":` + strconv.Quote(notePath) + `,"content":` + strconv.Quote(note) + `,"numLines":` + numLines + `,"startLine":1,"totalLines":` + numLines + `}}}`),
	}

	evidence := aicodingsession.NewEvidence(aicodingsession.Data{
		SchemaVersion: "0.1",
		RawSession:    map[string][]json.RawMessage{mainStream: raw},
	})

	adder := &fakeMaterialAdder{realDigests: true}
	attested, _ := attachSessionEvidence(context.Background(), adder, state.NewGitStore(t.TempDir()), []sessionEvidence{{
		sessionID: sessionID,
		provider:  claude.New(),
		evidence:  evidence,
		specs:     captures,
	}}, zerolog.Nop())
	require.Equal(t, []string{sessionID}, attested)

	material := adder.byName("spec-9a8b7c-design-note")
	assert.Contains(t, material.content, note, "the material holds the note, not the placeholder")
	assert.NotContains(t, material.content, spec.Placeholder)

	session := adder.byName("ai-coding-session-9a8b7c")
	assert.NotContains(t, session.content, "The export runs in the background", "no inline copy of the note")
	// Two copies in the write, two in the read, and the spec entry.
	assert.Equal(t, 5, strings.Count(session.content, material.digest), "each copy points to the material")
}

// TestReplaceSpecCopiesNeedsAFinder checks that the transcript of an agent
// that cannot find copies keeps them.
func TestReplaceSpecCopiesNeedsAFinder(t *testing.T) {
	const content = "the ticket"
	line := json.RawMessage(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"w","name":"Write","input":{"file_path":"/t.md","content":"` + content + `"}}]}}`)
	sources := pointer.Sources{pointer.Digest([]byte(content)): "sha256:material"}

	testCases := []struct {
		name     string
		provider trace.Provider
		want     bool
	}{
		{name: "claude code", provider: claude.New(), want: true},
		{name: "agent with no finder", provider: cursor.New(), want: false},
		{name: "no provider", provider: nil, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evidence := aicodingsession.NewEvidence(aicodingsession.Data{
				RawSession: map[string][]json.RawMessage{mainStream: {line}},
			})

			replaceSpecCopies(tc.provider, evidence, sources, "s", zerolog.Nop())

			assert.Equal(t, tc.want, !strings.Contains(string(evidence.Data.RawSession[mainStream][0]), content))
		})
	}
}
