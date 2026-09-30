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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addedMaterial is one call recorded by fakeMaterialAdder, with the file read
// back at call time: the caller is free to delete it once the add returns.
type addedMaterial struct {
	name, fileName, kind, content string
	annotations                   map[string]string
}

type fakeMaterialAdder struct {
	added []addedMaterial
	// failOn names the materials whose add fails.
	failOn map[string]bool
}

func (f *fakeMaterialAdder) AddMaterial(_ context.Context, name, path, kind string, annotations map[string]string) (string, error) {
	if f.failOn[name] {
		return "", errors.New("upload refused")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	f.added = append(f.added, addedMaterial{
		name: name, fileName: filepath.Base(path), kind: kind, content: string(content), annotations: annotations,
	})

	return "sha256:" + name, nil
}

// specCapture builds a capture the way the spec folder reader does: parsed from
// the file, with the file itself kept alongside.
func specCapture(t *testing.T, fileName, doc, capturedAt string) spec.Capture {
	t.Helper()

	at, err := time.Parse(time.RFC3339, capturedAt)
	require.NoError(t, err)

	c := spec.Parse([]byte(doc), at)
	require.NotNil(t, c)
	c.FileName = fileName
	c.Raw = []byte(doc)

	return *c
}

func TestAttachSpecs(t *testing.T) {
	const (
		sessionID = "7412a0c2-1a2b-4c3d-8e9f-0a1b2c3d4e5f"
		ticketURI = "https://linear.app/chainloop/issue/PFM-7289"
	)
	// Assembled from fragments so that nothing credential-shaped is committed.
	const pat = "ghp_erOZlZv0B1e3amrQ" + "ugdwZ8Ro2W4kDql9WPTf"

	ticket := specCapture(t, "ticket-pfm-7289.md", "---\nkind: ticket\nuri: "+ticketURI+"\n---\nthe ticket", "2026-09-16T10:12:03Z")
	plan := specCapture(t, "Approved Plan.md", "the plan", "2026-09-16T10:38:37Z")

	t.Run("each capture becomes an EVIDENCE material and a reference", func(t *testing.T) {
		adder := &fakeMaterialAdder{}

		entries, warnings := attachSpecs(context.Background(), adder, newSpecRedactor(t.TempDir()), materialNames{}, sessionID, []spec.Capture{ticket, plan}, zerolog.Nop())

		assert.Empty(t, warnings)
		require.Len(t, adder.added, 2)

		assert.Equal(t, "spec-7412a0-ticket-pfm-7289", adder.added[0].name)
		assert.Equal(t, "ticket-pfm-7289.md", adder.added[0].fileName, "the stored file keeps the name the agent chose")
		assert.Equal(t, "EVIDENCE", adder.added[0].kind)
		// The stored file is the file the agent wrote, header included.
		assert.Equal(t, "---\nkind: ticket\nuri: "+ticketURI+"\n---\nthe ticket", adder.added[0].content)
		assert.Equal(t, map[string]string{
			"chainloop.spec.session_id": sessionID,
			"chainloop.spec.kind":       aicodingsession.SpecKindTicket,
			"chainloop.spec.uri":        ticketURI,
		}, adder.added[0].annotations)

		// A material name admits only lowercase letters, digits and hyphens.
		assert.Equal(t, "spec-7412a0-approved-plan", adder.added[1].name)
		// No URI, no annotation for it.
		assert.NotContains(t, adder.added[1].annotations, specAnnotationURI)

		// Each reference carries the digest the material was stored under.
		assert.Equal(t, []aicodingsession.SpecEntry{
			{
				Kind: aicodingsession.SpecKindTicket, URI: ticketURI,
				Digest: "sha256:spec-7412a0-ticket-pfm-7289", CapturedAt: "2026-09-16T10:12:03Z",
			},
			{
				Kind: aicodingsession.SpecKindText, Digest: "sha256:spec-7412a0-approved-plan",
				CapturedAt: "2026-09-16T10:38:37Z",
			},
		}, entries)
	})

	t.Run("secrets are removed from the text and the source address before upload", func(t *testing.T) {
		adder := &fakeMaterialAdder{}
		withSecret := specCapture(t, "ticket.md",
			"---\nkind: ticket\nuri: https://tracker.example.com/issue/1?token="+pat+"\n---\nconfigured with the token "+pat+" and still a 401",
			"2026-09-16T10:12:03Z")

		entries, warnings := attachSpecs(context.Background(), adder, newSpecRedactor(t.TempDir()), materialNames{}, sessionID, []spec.Capture{withSecret}, zerolog.Nop())

		assert.Empty(t, warnings)
		require.Len(t, adder.added, 1)
		// The address is scanned like the text: a token in a URL must not
		// survive in the stored file, the annotation or the reference.
		wantURI := "https://tracker.example.com/issue/1?token=[REDACTED:github-pat]"
		assert.Equal(t, "---\nkind: ticket\nuri: "+wantURI+"\n---\nconfigured with the token [REDACTED:github-pat] and still a 401", adder.added[0].content)
		assert.Equal(t, wantURI, adder.added[0].annotations[specAnnotationURI])
		require.Len(t, entries, 1)
		assert.Equal(t, wantURI, entries[0].URI)
	})

	t.Run("a binary file is stored byte for byte and never scanned", func(t *testing.T) {
		adder := &fakeMaterialAdder{}
		scans := 0
		redactor := &specRedactor{dir: t.TempDir(), redact: func(_ context.Context, doc []byte) ([]byte, error) {
			scans++
			return doc, nil
		}}
		pngBytes := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\xff\xfe")
		image := spec.Capture{
			FileName: "mockup.png", Kind: aicodingsession.SpecKindImage,
			CapturedAt: "2026-09-16T10:31:40Z", Raw: pngBytes, Verbatim: true,
		}

		entries, warnings := attachSpecs(context.Background(), adder, redactor, materialNames{}, sessionID, []spec.Capture{image}, zerolog.Nop())

		assert.Empty(t, warnings)
		assert.Zero(t, scans, "a text scanner has nothing to read in an image, and a rewrite would break it")
		require.Len(t, adder.added, 1)
		assert.Equal(t, string(pngBytes), adder.added[0].content)
		assert.Equal(t, "mockup.png", adder.added[0].fileName)
		assert.Equal(t, aicodingsession.SpecKindImage, adder.added[0].annotations[specAnnotationKind])
		assert.NotContains(t, adder.added[0].annotations, specAnnotationURI)
		require.Len(t, entries, 1)
		assert.Equal(t, aicodingsession.SpecKindImage, entries[0].Kind)
	})

	t.Run("a failed add drops that entry only, and says so", func(t *testing.T) {
		adder := &fakeMaterialAdder{failOn: map[string]bool{"spec-7412a0-ticket-pfm-7289": true}}

		entries, warnings := attachSpecs(context.Background(), adder, newSpecRedactor(t.TempDir()), materialNames{}, sessionID, []spec.Capture{ticket, plan}, zerolog.Nop())

		// A reference to a material that is not in the attestation would point
		// nowhere, so the entry goes with it.
		require.Len(t, entries, 1)
		assert.Equal(t, "sha256:spec-7412a0-approved-plan", entries[0].Digest)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "ticket-pfm-7289.md")
		// The raw error stays in the local log: the warning goes into the
		// uploaded evidence.
		assert.NotContains(t, warnings[0], "upload refused")
	})

	t.Run("files that name the same material stay apart", func(t *testing.T) {
		adder := &fakeMaterialAdder{}
		a := ticket
		a.FileName = "Design.md"
		b := ticket
		b.FileName = "design.txt"
		// Its own name is the suffix the second file would get, so the suffix
		// must skip it.
		c := ticket
		c.FileName = "design-2.md"

		entries, _ := attachSpecs(context.Background(), adder, newSpecRedactor(t.TempDir()), materialNames{}, sessionID, []spec.Capture{a, b, c}, zerolog.Nop())

		require.Len(t, entries, 3)
		names := []string{adder.added[0].name, adder.added[1].name, adder.added[2].name}
		assert.Equal(t, []string{"spec-7412a0-design", "spec-7412a0-design-2", "spec-7412a0-design-2-2"}, names)
	})

	t.Run("sessions whose IDs share a prefix stay apart", func(t *testing.T) {
		// OpenCode session IDs all start with "ses_", so two of them often
		// share the six characters a material name takes from the ID.
		adder := &fakeMaterialAdder{}
		names := materialNames{}

		first := specCapture(t, "ticket.md", "the first ticket", "2026-09-16T10:12:03Z")
		second := specCapture(t, "ticket.md", "the second ticket", "2026-09-16T10:12:04Z")
		_, _ = attachSpecs(context.Background(), adder, newSpecRedactor(t.TempDir()), names, "ses_3AbC9x", []spec.Capture{first}, zerolog.Nop())
		_, _ = attachSpecs(context.Background(), adder, newSpecRedactor(t.TempDir()), names, "ses_3AbZ7y", []spec.Capture{second}, zerolog.Nop())

		require.Len(t, adder.added, 2)
		assert.NotEqual(t, adder.added[0].name, adder.added[1].name, "one attestation must never hold two materials under one name")
	})
}

func TestSpecMaterialName(t *testing.T) {
	const session = "abcdef123"

	testCases := []struct {
		name      string
		sessionID string
		fileName  string
		want      string
	}{
		{name: "a plain file name", sessionID: session, fileName: "ticket-eng-12.md", want: "spec-abcdef-ticket-eng-12"},
		{name: "case and separators are normalised", sessionID: session, fileName: "My Design_Doc v2.md", want: "spec-abcdef-my-design-doc-v2"},
		// OpenCode session IDs carry an underscore and upper-case letters.
		{name: "an OpenCode session ID", sessionID: "ses_3AbC9", fileName: "a.md", want: "spec-ses-3a-a"},
		{name: "nothing usable in the file name", sessionID: session, fileName: "___.md", want: "spec-abcdef-source"},
		{name: "a long file name is cut", sessionID: session, fileName: "an-extremely-long-file-name-that-goes-on-and-on-and-on.md", want: "spec-abcdef-an-extremely-long-file-name-that-goes-on"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, specMaterialName(tc.sessionID, tc.fileName))
		})
	}
}

func TestSpecRedactor(t *testing.T) {
	// Assembled from fragments so that nothing credential-shaped is committed.
	const pat = "ghp_erOZlZv0B1e3amrQ" + "ugdwZ8Ro2W4kDql9WPTf"

	t.Run("an unchanged file is not scanned again, even by a later push", func(t *testing.T) {
		dir := t.TempDir()
		scans := 0
		first := &specRedactor{dir: dir, redact: func(_ context.Context, doc []byte) ([]byte, error) {
			scans++
			return append([]byte("redacted: "), doc...), nil
		}}

		got, err := first.Redact(context.Background(), []byte("the ticket"))
		require.NoError(t, err)
		assert.Equal(t, "redacted: the ticket", string(got))

		// A later push builds its own redactor over the same directory.
		second := &specRedactor{dir: dir, redact: first.redact}
		got, err = second.Redact(context.Background(), []byte("the ticket"))
		require.NoError(t, err)
		assert.Equal(t, "redacted: the ticket", string(got))
		assert.Equal(t, 1, scans, "the second push reuses the stored copy")

		// The copy is found by the digest of the file.
		sum := sha256.Sum256([]byte("the ticket"))
		assert.FileExists(t, filepath.Join(dir, hex.EncodeToString(sum[:])))

		// Any change to the file is a new scan, a header change included.
		_, err = second.Redact(context.Background(), []byte("---\nkind: document\n---\nthe ticket"))
		require.NoError(t, err)
		assert.Equal(t, 2, scans)
	})

	t.Run("a failed scan stores nothing", func(t *testing.T) {
		dir := t.TempDir()
		failing := &specRedactor{dir: dir, redact: func(context.Context, []byte) ([]byte, error) {
			return nil, errors.New("scanner unavailable")
		}}

		_, err := failing.Redact(context.Background(), []byte("the ticket"))
		require.Error(t, err)

		entries, err := os.ReadDir(dir)
		if err == nil {
			assert.Empty(t, entries, "text that was never scanned must not be reused as if it had been")
		}
	})

	t.Run("the default scanner removes secrets", func(t *testing.T) {
		got, err := newSpecRedactor(t.TempDir()).Redact(context.Background(), []byte("the token "+pat+" fails"))
		require.NoError(t, err)
		assert.Equal(t, "the token [REDACTED:github-pat] fails", string(got))
	})
}
