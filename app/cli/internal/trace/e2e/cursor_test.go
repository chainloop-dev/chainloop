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
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/cursor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCursorVersion(t *testing.T) {
	cases := []struct {
		name    string
		out     string
		want    string
		wantErr bool
	}{
		{name: "build tag", out: "2026.01.09-6d1b8f1\n", want: "2026.01.09-6d1b8f1"},
		{name: "empty", out: "\n", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCursorVersion([]byte(tc.out))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCursorError(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name: "success",
			raw: `{"type":"system","subtype":"init","session_id":"s1"}
{"type":"result","subtype":"success","is_error":false,"result":"done"}
`,
		},
		{
			name:    "error result",
			raw:     `{"type":"result","subtype":"error","is_error":true,"result":"invalid API key"}` + "\n",
			wantErr: "invalid API key",
		},
		{
			name:    "line that is not JSON",
			raw:     "not json\n",
			wantErr: "decode cursor-agent event",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := cursorError([]byte(tc.raw))
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestWriteCursorShellProfiles(t *testing.T) {
	cases := []struct {
		name string
		env  []string
		want string
	}{
		{name: "plain path", env: []string{"PATH=/e2e/bin:/usr/bin"}, want: "/e2e/bin:/usr/bin"},
		{name: "last PATH wins", env: []string{"PATH=/old", "PATH=/e2e/bin"}, want: "/e2e/bin"},
		{name: "quote in path", env: []string{"PATH=/it's/bin:/usr/bin"}, want: "/it's/bin:/usr/bin"},
		{name: "no PATH"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "cursor-home")
			require.NoError(t, writeCursorShellProfiles(home, tc.env))
			require.DirExists(t, home)

			for _, name := range cursorShellProfiles {
				path := filepath.Join(home, name)
				if tc.want == "" {
					assert.NoFileExists(t, path)
					continue
				}
				out, err := exec.Command("/bin/sh", "-c", `. "$1"; printf %s "$PATH"`, "sh", path).Output()
				require.NoError(t, err)
				assert.Equal(t, tc.want, string(out), name)
			}
		})
	}
}

func TestCursorRunWithImages(t *testing.T) {
	c := &Cursor{}
	_, err := c.Run(t.Context(), RunRequest{Dir: t.TempDir(), Images: [][]byte{{1}}})
	assert.ErrorIs(t, err, errCursorImages)
}

func TestCursorDisableSessionStart(t *testing.T) {
	c := &Cursor{}

	t.Run("removes only the session start hook", func(t *testing.T) {
		repo := t.TempDir()
		hc := c.HookConfig()
		path := filepath.Join(repo, hc.Path)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(hc.UserConfig), 0o600))
		require.NoError(t, cursor.New().InstallHooks(repo))

		require.NoError(t, c.DisableSessionStart(repo))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		var settings struct {
			Version int                        `json:"version"`
			Hooks   map[string]json.RawMessage `json:"hooks"`
		}
		require.NoError(t, json.Unmarshal(data, &settings))
		assert.Equal(t, 1, settings.Version)
		assert.NotContains(t, settings.Hooks, cursorSessionStartEvent)
		assert.Contains(t, settings.Hooks, "sessionEnd")
		assert.Contains(t, settings.Hooks, "stop")
		assert.Contains(t, string(data), hc.UserMarker)
	})

	t.Run("no session start hook", func(t *testing.T) {
		repo := t.TempDir()
		path := filepath.Join(repo, cursorHooksPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(c.HookConfig().UserConfig), 0o600))

		assert.Error(t, c.DisableSessionStart(repo))
	})
}
