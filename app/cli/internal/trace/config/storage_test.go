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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTraceStorage(t *testing.T) {
	testCases := []struct {
		name      string
		persisted string
		want      string
		wantErr   bool
	}{
		{name: "empty defaults to control plane", persisted: "", want: TraceStorageControlPlane},
		{name: "control plane", persisted: "CONTROL_PLANE", want: TraceStorageControlPlane},
		{name: "local", persisted: "LOCAL", want: TraceStorageLocal},
		{name: "lower-case is canonicalized", persisted: "local", want: TraceStorageLocal},
		{name: "surrounding space is trimmed", persisted: "  LOCAL  ", want: TraceStorageLocal},
		{name: "mixed case", persisted: "Control_Plane", want: TraceStorageControlPlane},
		{name: "unknown value is an error", persisted: "LOCALL", wantErr: true},
		{name: "a non-empty typo does not fall back", persisted: "disk", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveTraceStorage(tc.persisted)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLoadTraceStorageFromYML(t *testing.T) {
	t.Run("missing config is control plane", func(t *testing.T) {
		mode, err := LoadTraceStorageFromYML(t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, TraceStorageControlPlane, mode)
	})

	t.Run("reads and canonicalizes local", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\ntraceStorage: local\n"), 0600))

		mode, err := LoadTraceStorageFromYML(dir)
		require.NoError(t, err)
		assert.Equal(t, TraceStorageLocal, mode)
		assert.True(t, IsLocalStorage(dir))
	})

	t.Run("invalid value surfaces an error", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("traceStorage: nope\n"), 0600))

		_, err := LoadTraceStorageFromYML(dir)
		assert.Error(t, err)
		// Best-effort callers treat an invalid value as not-local.
		assert.False(t, IsLocalStorage(dir))
	})

	t.Run("a malformed config is an error, not a silent control plane", func(t *testing.T) {
		// A LOCAL repository whose config broke must not fall back to the
		// control plane and upload a local-only session: the pre-push hook
		// reads this and a silent CONTROL_PLANE would do exactly that.
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("traceStorage: [not, a, string]\n"), 0600))

		mode, err := LoadTraceStorageFromYML(dir)
		assert.Error(t, err)
		assert.NotEqual(t, TraceStorageControlPlane, mode)
	})
}

func TestTraceStorageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
		[]byte("projectName: p\nprojectVersion: v1.0.0\n"), 0600))

	require.NoError(t, SaveTraceStorageToYML(dir, TraceStorageLocal))
	require.NoError(t, SaveTracePathToYML(dir, "evidence/sessions"))

	assert.True(t, IsLocalStorage(dir))
	assert.Equal(t, "evidence/sessions", LoadTracePathFromYML(dir))

	// Other fields survive.
	data, err := os.ReadFile(filepath.Join(dir, ".chainloop.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "projectName: p")
	assert.Contains(t, string(data), "projectVersion: v1.0.0")
}

func TestLoadTraceRequiredFromYML(t *testing.T) {
	testCases := []struct {
		name string
		yml  string
		want bool
	}{
		{name: "absent in both is false", yml: "projectName: p\n", want: false},
		{name: "traceRequired true", yml: "traceRequired: true\n", want: true},
		{name: "traceRequired false", yml: "traceRequired: false\n", want: false},
		{name: "falls back to deprecated requireTrace", yml: "requireTrace: true\n", want: true},
		{
			name: "traceRequired wins over requireTrace",
			yml:  "traceRequired: false\nrequireTrace: true\n",
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte(tc.yml), 0600))
			assert.Equal(t, tc.want, LoadTraceRequiredFromYML(dir))
		})
	}

	t.Run("no config file is false", func(t *testing.T) {
		assert.False(t, LoadTraceRequiredFromYML(t.TempDir()))
	})

	t.Run("save writes traceRequired and round-trips", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("projectName: p\n"), 0600))

		require.NoError(t, SaveTraceRequiredToYML(dir, true))
		assert.True(t, LoadTraceRequiredFromYML(dir))

		data, err := os.ReadFile(filepath.Join(dir, ".chainloop.yml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "traceRequired: true")
	})
}
