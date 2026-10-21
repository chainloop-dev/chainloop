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

//nolint:goconst // Keep native protocol literals readable in regression fixtures.
package action

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/codex"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexMaterialAssembly(t *testing.T) {
	const sessionID = "c0de42-session"
	const task = "# Task\n\nImplement the requested change.\n"
	const definition = "---\nname: example\ndescription: Test skill.\n---\n\nRead reference.txt.\n"
	image := []byte("\x89PNG\r\n\x1a\nscreenshot")
	encodedImage := base64.StdEncoding.EncodeToString(image)
	for _, tc := range []struct {
		name  string
		reuse bool
		fail  bool
	}{
		{name: "materials and viewer pointers"},
		{name: "captured image wins", reuse: true},
		{name: "storage failure preserves inline image", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("CODEX_HOME", home)
			root := t.TempDir()
			skillDir := filepath.Join(root, ".agents", "skills", "example")
			require.NoError(t, os.MkdirAll(skillDir, 0700))
			require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(definition), 0600))
			reference := filepath.Join(skillDir, "reference.txt")
			require.NoError(t, os.WriteFile(reference, []byte("original reference"), 0600))
			var rollout bytes.Buffer
			add := func(kind string, payload any) {
				t.Helper()
				require.NoError(t, json.NewEncoder(&rollout).Encode(struct {
					Type      string `json:"type"`
					Timestamp string `json:"timestamp"`
					Payload   any    `json:"payload"`
				}{Type: kind, Timestamp: "2026-10-21T10:00:00Z", Payload: payload}))
			}
			add("session_meta", map[string]string{"id": sessionID, "cwd": root, "cli_version": "0.163.0"})
			add("response_item", map[string]any{"type": "message", "role": "user", "content": []map[string]string{
				{"type": "input_image", "image_url": "data:image/png;base64," + encodedImage},
				{"type": "input_image", "image_url": "data:image/png;base64," + encodedImage},
			}})
			args, err := json.Marshal(map[string]string{"cmd": "cat " + filepath.Join(skillDir, "SKILL.md")})
			require.NoError(t, err)
			add("response_item", map[string]string{"type": "function_call", "name": "exec_command", "call_id": "skill", "arguments": string(args)})
			add("response_item", map[string]string{"type": "function_call_output", "call_id": "skill", "output": definition})
			add("response_item", map[string]string{"type": "function_call", "name": "exec_command", "call_id": "task", "arguments": `{"cmd":"cat task.md"}`})
			add("response_item", map[string]string{"type": "function_call_output", "call_id": "task", "output": task})
			path := filepath.Join(home, "sessions", "rollout-"+sessionID+".jsonl")
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
			require.NoError(t, os.WriteFile(path, rollout.Bytes(), 0600))
			provider := codex.New()
			store := state.NewOutOfTreeStore(t.TempDir())
			captureSkillLoads(provider, store, &trace.HookInput{SessionID: sessionID, TranscriptPath: path}, zerolog.Nop())
			// The first-use snapshot, including supporting files, must survive
			// later edits of the live package.
			require.NoError(t, os.WriteFile(reference, []byte("changed after load"), 0600))
			require.NoError(t, provider.CopySessionData(store, trace.SessionLocation{SessionID: sessionID, TranscriptPath: path}))
			opts := &trace.ParseOpts{SessionID: sessionID, SessionDir: store.RawSessionDir()}
			evidence, err := provider.ParseSession(t.Context(), opts)
			require.NoError(t, err)
			skills, warnings := readSessionSkills(provider, store, opts, root, zerolog.Nop())
			require.Empty(t, warnings)
			require.Len(t, skills, 1)
			captures := []spec.Capture{specCapture(t, "task.md", "---\nkind: ticket\ntitle: Task\n---\n"+task, "2026-10-21T10:00:00Z")}
			captures[0].SourceDigest = pointer.Digest([]byte(task))
			if tc.reuse {
				captures = append(captures, spec.Capture{FileName: "reference.png", Kind: aicodingsession.SpecKindImage, Raw: image, Verbatim: true})
			}
			adder := &fakeMaterialAdder{realDigests: true}
			if tc.fail {
				adder.failOn = map[string]bool{"spec-c0de42-pasted-image-1": true}
			}
			attested, _ := attachSessionEvidence(t.Context(), adder, store, []sessionEvidence{{sessionID: sessionID, provider: provider, evidence: evidence, specs: captures, skills: skills}}, zerolog.Nop(), true)
			require.Equal(t, []string{sessionID}, attested)
			var result aicodingsession.Evidence
			session := adder.byName("ai-coding-session-c0de42")
			require.NoError(t, json.Unmarshal([]byte(session.content), &result))
			assert.Equal(t, tc.fail, bytes.Contains([]byte(session.content), []byte(encodedImage)))
			imageCount := 0
			for _, entry := range result.Data.Spec {
				switch entry.Kind {
				case aicodingsession.SpecKindImage:
					imageCount++
					assert.Contains(t, session.content, entry.Digest)
				case aicodingsession.SpecKindSkill:
					require.NotNil(t, entry.Metadata)
					assert.Equal(t, 1, entry.Metadata.ByModel)
					archive := adder.byName("spec-c0de42-skill-example-pkg")
					require.Equal(t, entry.Metadata.PackageDigest, archive.digest)
					compressed, err := gzip.NewReader(bytes.NewReader([]byte(archive.content)))
					require.NoError(t, err)
					defer compressed.Close()
					reader := tar.NewReader(compressed)
					found := false
					for {
						header, err := reader.Next()
						if err == io.EOF {
							break
						}
						require.NoError(t, err)
						if filepath.Base(header.Name) == "reference.txt" {
							content, err := io.ReadAll(reader)
							require.NoError(t, err)
							assert.Equal(t, "original reference", string(content))
							found = true
						}
					}
					assert.True(t, found)
				}
			}
			if tc.fail {
				assert.Zero(t, imageCount)
			} else {
				assert.Equal(t, 1, imageCount)
			}
			assert.Contains(t, string(result.Data.RawSession["main"][5]), pointer.Marker)
		})
	}
}
