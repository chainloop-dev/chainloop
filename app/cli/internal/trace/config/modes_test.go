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

func TestParseTraceModes(t *testing.T) {
	testCases := []struct {
		name    string
		values  []string
		want    TraceModes
		wantErr bool
	}{
		{name: "absent defaults to online", values: nil, want: TraceModes{Online: true}},
		{name: "empty list defaults to online", values: []string{}, want: TraceModes{Online: true}},
		{name: "online", values: []string{"online"}, want: TraceModes{Online: true}},
		{name: "offline", values: []string{"offline"}, want: TraceModes{Offline: true}},
		{name: "both", values: []string{"online", "offline"}, want: TraceModes{Online: true, Offline: true}},
		{name: "order does not matter", values: []string{"offline", "online"}, want: TraceModes{Online: true, Offline: true}},
		{name: "case and space are canonicalized", values: []string{" OFFLINE ", "Online"}, want: TraceModes{Online: true, Offline: true}},
		{name: "a repeated mode is one mode", values: []string{"offline", "offline"}, want: TraceModes{Offline: true}},
		{name: "an unknown mode is an error", values: []string{"local"}, wantErr: true},
		{name: "an empty entry is an error", values: []string{"online", ""}, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTraceModes(tc.values)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestTraceModesNames(t *testing.T) {
	assert.Equal(t, []string{"online"}, TraceModes{Online: true}.Names())
	assert.Equal(t, []string{"offline"}, TraceModes{Offline: true}.Names())
	assert.Equal(t, []string{"online", "offline"}, TraceModes{Online: true, Offline: true}.Names())
	assert.Empty(t, TraceModes{}.Names())
}

func TestLoadTraceModesFromYML(t *testing.T) {
	testCases := []struct {
		name    string
		yml     string
		want    TraceModes
		wantErr bool
	}{
		{name: "no field is online", yml: "projectName: p\n", want: TraceModes{Online: true}},
		{name: "offline only", yml: "traceModes:\n  - offline\n", want: TraceModes{Offline: true}},
		{name: "both in flow style", yml: "traceModes: [online, offline]\n", want: TraceModes{Online: true, Offline: true}},
		{name: "an unknown mode is an error", yml: "traceModes: [lcoal]\n", wantErr: true},
		// An offline repository whose config broke must not fall back to
		// online and upload an offline-only session: the pre-push hook reads
		// this, and a silent online default would do exactly that.
		{name: "a malformed config is an error, not a silent online", yml: "traceModes: {not: a list}\n", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte(tc.yml), 0600))

			got, err := LoadTraceModesFromYML(dir)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("no config file is online", func(t *testing.T) {
		got, err := LoadTraceModesFromYML(t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, TraceModes{Online: true}, got)
	})
}

func TestTraceModesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
		[]byte("projectName: p\nprojectVersion: v1.0.0\n"), 0600))

	require.NoError(t, SaveTraceModesToYML(dir, TraceModes{Online: true, Offline: true}))
	require.NoError(t, SaveTracePathToYML(dir, "evidence/sessions"))

	got, err := LoadTraceModesFromYML(dir)
	require.NoError(t, err)
	assert.Equal(t, TraceModes{Online: true, Offline: true}, got)
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
