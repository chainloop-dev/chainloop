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

package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenCodeRunOutput(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantID    string
		wantIDErr bool
		wantErr   string
	}{
		{
			name: "session",
			raw: `{"type":"step_start","sessionID":"ses_1","part":{}}
{"type":"text","sessionID":"ses_1","part":{"text":"done"}}
`,
			wantID: "ses_1",
		},
		{
			name: "error event",
			raw: `{"type":"step_start","sessionID":"ses_2","part":{}}
{"type":"error","sessionID":"ses_2","error":{"type":"provider.auth","message":"Token is expired."}}
`,
			wantID:  "ses_2",
			wantErr: "provider.auth: Token is expired.",
		},
		{
			name:      "no session",
			raw:       "\n",
			wantIDErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := openCodeError([]byte(tc.raw))
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			id, err := openCodeSessionID([]byte(tc.raw))
			if tc.wantIDErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantID, id)
		})
	}
}

func TestParseOpenCodeHookMessages(t *testing.T) {
	cases := []struct {
		name   string
		export string
		want   []HookMessage
	}{
		{
			name: "instruction and reminder",
			export: `{"info":{"id":"ses_1"},"messages":[
{"type":"synthetic","text":"Chainloop spec capture. Folder: /x\nCapture the specs."},
{"type":"synthetic","text":"Chainloop spec capture. Folder: /x\n` + openCodeReminderMarker + `, one file per source."},
{"type":"user","text":"do it"},
{"type":"assistant","text":""}]}`,
			want: []HookMessage{
				{Event: "SessionStart", ModelContext: "Chainloop spec capture. Folder: /x\nCapture the specs."},
				{Event: "UserPromptSubmit", ModelContext: "Chainloop spec capture. Folder: /x\n" + openCodeReminderMarker + ", one file per source."},
			},
		},
		{
			name:   "no synthetic message",
			export: `{"info":{"id":"ses_1"},"messages":[{"type":"user","text":"do it"}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOpenCodeHookMessages([]byte(tc.export))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestOpenCodeDisableSessionStart(t *testing.T) {
	cases := []struct {
		name    string
		plugin  string
		want    string
		wantErr bool
	}{
		{
			name:   "handler",
			plugin: "async function sessionEvent() {\n  " + openCodeSessionStartHandler + "\n    await start()\n  }\n}\n",
			want:   "async function sessionEvent() {\n  if (type === \"e2e.disabled\") {\n    await start()\n  }\n}\n",
		},
		{
			name:    "no handler",
			plugin:  "export default {}\n",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			path := filepath.Join(repo, openCodePluginPath)
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
			require.NoError(t, os.WriteFile(path, []byte(tc.plugin), 0o600))

			err := (&OpenCode{Major: "2"}).DisableSessionStart(repo)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}
}
