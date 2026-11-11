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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pi"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/skill"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestPiSkillSnapshotAtUseAndPushFallback(t *testing.T) {
	for _, tc := range []struct {
		name   string
		atUse  bool
		reload bool
	}{
		{name: "first use", atUse: true},
		{name: "changed later use", atUse: true, reload: true},
		{name: "push fallback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			repo, gitDir := initGitRepo(t)
			store := state.NewGitStore(gitDir)
			require.NoError(t, store.InitTraceDir())
			const sessionID = "pi-skill-snapshot"
			provider := pi.New()
			skillDir := filepath.Join(repo, ".agents", "skills", "review")
			require.NoError(t, os.MkdirAll(skillDir, 0o700))
			original := "---\nname: review\ndescription: Review code.\n---\nOriginal instructions.\n"
			modified := "---\nname: review\ndescription: Review code.\n---\nChanged after the skill was loaded.\n"
			definition := filepath.Join(skillDir, skill.DefinitionFile)
			require.NoError(t, os.WriteFile(definition, []byte(original), 0o600))
			script := filepath.Join(skillDir, "check.sh")
			require.NoError(t, os.WriteFile(script, []byte("original script\n"), 0o600))
			transcript := filepath.Join(t.TempDir(), "native.jsonl")
			body := fmt.Sprintf(`{"type":"session","version":3,"id":%q,"cwd":%q,"timestamp":"2026-10-30T10:00:00Z"}
{"type":"message","id":"call","parentId":null,"timestamp":"2026-10-30T10:00:00Z","message":{"role":"assistant","content":[{"type":"toolCall","id":"load","name":"read","arguments":{"path":%q}}]}}
{"type":"message","id":"result","parentId":"call","timestamp":"2026-10-30T10:00:01Z","message":{"role":"toolResult","toolName":"read","toolCallId":"load","content":[{"type":"text","text":%q}]}}
`, sessionID, repo, definition, original)
			require.NoError(t, os.WriteFile(transcript, []byte(body), 0o600))

			// When
			capture := func() {
				captureSkillLoads(provider, store, &trace.HookInput{
					SessionID: sessionID, TranscriptPath: transcript,
					ToolName: "write", FilePath: filepath.Join(repo, "changed.go"),
				}, zerolog.Nop())
			}
			if tc.atUse {
				capture()
			}
			require.NoError(t, os.WriteFile(definition, []byte(modified), 0o600))
			require.NoError(t, os.WriteFile(script, []byte("changed script\n"), 0o600))
			if tc.reload {
				reloaded := fmt.Sprintf(`{"type":"message","id":"reload","parentId":"result","timestamp":"2026-10-30T10:01:00Z","message":{"role":"assistant","content":[{"type":"toolCall","id":"load-again","name":"read","arguments":{"path":%q}}]}}
{"type":"message","id":"reloaded","parentId":"reload","timestamp":"2026-10-30T10:01:01Z","message":{"role":"toolResult","toolName":"read","toolCallId":"load-again","content":[{"type":"text","text":%q}]}}
`, definition, modified)
				require.NoError(t, os.WriteFile(transcript, []byte(body+reloaded), 0o600))
				capture()
			}
			require.NoError(t, provider.CopySessionData(store, trace.SessionLocation{SessionID: sessionID, TranscriptPath: transcript}))
			opts := &trace.ParseOpts{SessionID: sessionID, SessionDir: store.RawSessionDir()}
			uses, _, err := provider.SessionSkills(opts)
			require.NoError(t, err)
			require.Len(t, uses, 1)
			definitions, warnings := readSessionSkills(provider, store, opts, repo, zerolog.Nop())

			// Then
			require.Len(t, definitions, 1, "%v", warnings)
			snapshot := definitions[0]
			require.Equal(t, "project", snapshot.source)
			if tc.reload {
				require.Equal(t, 2, snapshot.use.ByModel)
			} else {
				require.Equal(t, 1, snapshot.use.ByModel)
			}
			copied, err := os.ReadFile(filepath.Join(snapshot.copyDir, "files", skill.DefinitionFile))
			require.NoError(t, err)
			copiedScript, err := os.ReadFile(filepath.Join(snapshot.copyDir, "files", "check.sh"))
			require.NoError(t, err)
			if tc.atUse {
				require.Equal(t, original, string(copied))
				require.Equal(t, "original script\n", string(copiedScript))
				if tc.reload {
					require.Len(t, warnings, 1)
				} else {
					require.Empty(t, warnings)
				}
			} else {
				require.Equal(t, modified, string(copied))
				require.Equal(t, "changed script\n", string(copiedScript))
				require.Len(t, warnings, 1)
			}
		})
	}
}
