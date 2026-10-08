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
	"strconv"
	"strings"
	"testing"

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

const (
	schemaVersion = "0.1"
	pastedTitle1  = "Pasted image 1"
)

// promptBlock is a block of a user prompt: text, or an image with inline data.
type promptBlock struct {
	Type   string       `json:"type"`
	Text   string       `json:"text,omitempty"`
	Source *imageSource `json:"source,omitempty"`
}

type imageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// pastedLine is a user prompt with pasted images, as Claude Code writes it.
func pastedLine(t *testing.T, text, timestamp string, pasteIDs []int, images ...[]byte) json.RawMessage {
	t.Helper()

	content := make([]promptBlock, 0, 1+len(images))
	content = append(content, promptBlock{Type: "text", Text: text})
	for _, img := range images {
		content = append(content, promptBlock{Type: "image", Source: &imageSource{
			Type: "base64", MediaType: "image/png", Data: base64.StdEncoding.EncodeToString(img),
		}})
	}

	type message struct {
		Role    string        `json:"role"`
		Content []promptBlock `json:"content"`
	}
	e := struct {
		Type          string  `json:"type"`
		Timestamp     string  `json:"timestamp"`
		Message       message `json:"message"`
		ImagePasteIDs []int   `json:"imagePasteIds,omitempty"`
	}{Type: "user", Timestamp: timestamp, Message: message{Role: "user", Content: content}, ImagePasteIDs: pasteIDs}

	out, err := json.Marshal(e)
	require.NoError(t, err)

	return out
}

// pastedSource returns the source of the image block at index block of the
// transcript entry.
func pastedSource(t *testing.T, raw json.RawMessage, block int) map[string]any {
	t.Helper()

	var e struct {
		Message struct {
			Content []struct {
				Type   string         `json:"type"`
				Source map[string]any `json:"source"`
			} `json:"content"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Less(t, block, len(e.Message.Content))
	require.Equal(t, "image", e.Message.Content[block].Type, "the block keeps its type")

	return e.Message.Content[block].Source
}

func pointerTo(digest string) map[string]any {
	return map[string]any{"type": pointer.Marker, "digest": digest}
}

// TestAttachSessionEvidencePastedImages follows R-001 to R-004 of spec
// issue-3569.
func TestAttachSessionEvidencePastedImages(t *testing.T) {
	const sessionID = "3569ab-session"

	screenshot := []byte("\x89PNG\r\n\x1a\nscreenshot of the bug")
	mockup := []byte("\x89PNG\r\n\x1a\nmockup of the page")
	ticketText := "---\nkind: ticket\nuri: https://tracker.example.com/ENG-1\ntitle: Add an export button\n---\n\nThe export button is missing.\n"

	attach := func(t *testing.T, adder *fakeMaterialAdder, captures []spec.Capture, lines ...json.RawMessage) aicodingsession.Evidence {
		t.Helper()

		evidence := aicodingsession.NewEvidence(aicodingsession.Data{
			SchemaVersion: schemaVersion,
			RawSession:    map[string][]json.RawMessage{mainStream: lines},
		})
		attested, _ := attachSessionEvidence(context.Background(), adder, state.NewGitStore(t.TempDir()), []sessionEvidence{{
			sessionID: sessionID,
			provider:  claude.New(),
			evidence:  evidence,
			specs:     captures,
		}}, zerolog.Nop(), false)
		require.Equal(t, []string{sessionID}, attested)

		var got aicodingsession.Evidence
		require.NoError(t, json.Unmarshal([]byte(adder.byName("ai-coding-session-3569ab").content), &got))

		return got
	}

	t.Run("a pasted image that the agent did not capture is stored and replaced", func(t *testing.T) {
		adder := &fakeMaterialAdder{realDigests: true}
		got := attach(t, adder, nil,
			pastedLine(t, "[Image #1] add this image in the readme", "2026-10-08T10:15:30.123Z", []int{1}, screenshot))

		material := adder.byName("spec-3569ab-pasted-image-1")
		require.NotEmpty(t, material.digest)
		assert.Equal(t, string(screenshot), material.content, "the material holds the decoded image byte for byte")
		assert.Equal(t, "EVIDENCE", material.kind)
		assert.Equal(t, map[string]string{
			specAnnotationSession: sessionID,
			specAnnotationKind:    aicodingsession.SpecKindImage,
			specAnnotationTitle:   pastedTitle1,
		}, material.annotations)

		assert.Equal(t, []aicodingsession.SpecEntry{{
			Kind: aicodingsession.SpecKindImage, Title: pastedTitle1,
			Digest: material.digest, CapturedAt: "2026-10-08T10:15:30Z",
		}}, got.Data.Spec)

		assert.Equal(t, pointerTo(material.digest), pastedSource(t, got.Data.RawSession[mainStream][0], 1))
		assert.NotContains(t, adder.byName("ai-coding-session-3569ab").content, base64.StdEncoding.EncodeToString(screenshot))
	})

	t.Run("the entry of the agent wins over the pasted image", func(t *testing.T) {
		adder := &fakeMaterialAdder{realDigests: true}
		meta := []byte("role: reference\ntitle: Screenshot of the bug\ndescription: The page with no export button.\n")
		captured := spec.Capture{
			FileName: "screenshot.png", Kind: aicodingsession.SpecKindImage,
			CapturedAt: "2026-10-08T10:16:00Z", Raw: screenshot, Verbatim: true, MetaRaw: meta,
		}

		got := attach(t, adder, []spec.Capture{captured},
			pastedLine(t, "[Image #1] add this image in the readme", "2026-10-08T10:15:30Z", []int{1}, screenshot))

		assert.Equal(t, []string{"spec-3569ab-screenshot", "ai-coding-session-3569ab"}, adder.names(), "no second material for the same bytes")
		material := adder.byName("spec-3569ab-screenshot")

		require.Len(t, got.Data.Spec, 1)
		assert.Equal(t, aicodingsession.SpecRoleReference, got.Data.Spec[0].Role)
		assert.Equal(t, "Screenshot of the bug", got.Data.Spec[0].Title)
		assert.Equal(t, material.digest, got.Data.Spec[0].Digest)

		assert.Equal(t, pointerTo(material.digest), pastedSource(t, got.Data.RawSession[mainStream][0], 1))
	})

	t.Run("two different pasted images and one pasted two times", func(t *testing.T) {
		adder := &fakeMaterialAdder{realDigests: true}
		got := attach(t, adder, []spec.Capture{specCapture(t, "ticket.md", ticketText, "2026-10-08T10:00:00Z")},
			pastedLine(t, "[Image #1] the bug", "2026-10-08T10:15:30Z", []int{1}, screenshot),
			pastedLine(t, "[Image #2] the mockup", "2026-10-08T10:20:00Z", []int{2}, mockup),
			pastedLine(t, "[Image #3] again the bug", "2026-10-08T10:25:00Z", []int{3}, screenshot),
		)

		// The entries of the agent come first.
		assert.Equal(t, []string{"spec-3569ab-ticket", "spec-3569ab-pasted-image-1", "spec-3569ab-pasted-image-2", "ai-coding-session-3569ab"}, adder.names())

		titles := make([]string, 0, len(got.Data.Spec))
		for _, e := range got.Data.Spec {
			titles = append(titles, e.Title)
			if strings.HasPrefix(e.Title, "Pasted image") {
				assert.Empty(t, e.Role)
				assert.Empty(t, e.Description)
				assert.Empty(t, e.URI)
			}
		}
		assert.Equal(t, []string{"Add an export button", pastedTitle1, "Pasted image 2"}, titles)
		assert.Equal(t, "2026-10-08T10:20:00Z", got.Data.Spec[2].CapturedAt)

		lines := got.Data.RawSession[mainStream]
		first := adder.byName("spec-3569ab-pasted-image-1").digest
		assert.Equal(t, pointerTo(first), pastedSource(t, lines[0], 1))
		assert.Equal(t, pointerTo(adder.byName("spec-3569ab-pasted-image-2").digest), pastedSource(t, lines[1], 1))
		assert.Equal(t, pointerTo(first), pastedSource(t, lines[2], 1))
	})

	t.Run("a pasted image that cannot be stored stays inline", func(t *testing.T) {
		adder := &fakeMaterialAdder{realDigests: true, failOn: map[string]bool{"spec-3569ab-pasted-image-1": true}}
		got := attach(t, adder, nil,
			pastedLine(t, "[Image #1]", "2026-10-08T10:15:30Z", []int{1}, screenshot))

		assert.Empty(t, got.Data.Spec)
		assert.Equal(t, []string{`spec entry "pasted-image-1.png" was not recorded`}, got.Data.Warnings)
		assert.Equal(t, "base64", pastedSource(t, got.Data.RawSession[mainStream][0], 1)["type"])
	})
}

// TestPastedImagesNeedAFinder checks that an agent with no pasted image
// finder keeps its pasted images inline and adds no entry for them.
func TestPastedImagesNeedAFinder(t *testing.T) {
	img := []byte("\x89PNG\r\n\x1a\npasted")
	evidence := aicodingsession.NewEvidence(aicodingsession.Data{
		RawSession: map[string][]json.RawMessage{mainStream: {pastedLine(t, "[Image #1]", "2026-10-08T10:15:30Z", []int{1}, img)}},
	})

	adder := &fakeMaterialAdder{realDigests: true}
	attachSessionEvidence(context.Background(), adder, state.NewGitStore(t.TempDir()), []sessionEvidence{{
		sessionID: "c0ffee-session", provider: cursor.New(), evidence: evidence,
	}}, zerolog.Nop(), false)

	assert.Equal(t, []string{"ai-coding-session-c0ffee"}, adder.names())
	assert.Contains(t, adder.byName("ai-coding-session-c0ffee").content, strconv.Quote(base64.StdEncoding.EncodeToString(img)))
}
