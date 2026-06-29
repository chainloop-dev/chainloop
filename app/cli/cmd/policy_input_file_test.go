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

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePolicyInputFiles(t *testing.T) {
	testCases := []struct {
		name    string
		raw     []string
		want    []*action.PolicyInputFromFile
		wantNil bool
		wantErr bool
	}{
		{
			name:    "nil input returns nil",
			raw:     nil,
			wantNil: true,
		},
		{
			name:    "empty input returns nil",
			raw:     []string{},
			wantNil: true,
		},
		{
			name:    "malformed value propagates the parse error",
			raw:     []string{"missing-equals"},
			wantErr: true,
		},
		{
			name: "scheme-less missing path keeps the original file value",
			raw:  []string{"ignored_paths=/does/not/exist.csv:Path"},
			want: []*action.PolicyInputFromFile{
				{Input: "ignored_paths", Column: "Path", File: "/does/not/exist.csv"},
			},
		},
		{
			name: "multiple entries keep order and default the column",
			raw: []string{
				"ignored_paths=/no/exist1.csv",
				"paths=/no/exist2.json:Glob",
			},
			want: []*action.PolicyInputFromFile{
				{Input: "ignored_paths", Column: "ignored_paths", File: "/no/exist1.csv"},
				{Input: "paths", Column: "Glob", File: "/no/exist2.json"},
			},
		},
		{
			name:    "unresolvable env reference errors",
			raw:     []string{"ignored_paths=env://CHAINLOOP_TEST_DEFINITELY_UNSET_VAR"},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolvePolicyInputFiles(tc.raw)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.wantNil {
				assert.Nil(t, got)
				return
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestResolvePolicyInputFilesExistingFile checks that an on-disk file is
// resolved to its own path (no temporary copy).
func TestResolvePolicyInputFilesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exception.csv")
	require.NoError(t, os.WriteFile(path, []byte("Path\nc:\\a.dll\n"), 0600))

	got, err := resolvePolicyInputFiles([]string{"ignored_paths=" + path + ":Path"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, &action.PolicyInputFromFile{Input: "ignored_paths", Column: "Path", File: path}, got[0])
}

// TestResolvePolicyInputFilesResolvesEnv checks that a non-file reference (here
// env://) is downloaded to a local temporary path that differs from the
// original reference.
func TestResolvePolicyInputFilesResolvesEnv(t *testing.T) {
	t.Setenv("CHAINLOOP_TEST_POLICY_INPUT", `["c:\\a.dll"]`)

	got, err := resolvePolicyInputFiles([]string{"ignored_paths=env://CHAINLOOP_TEST_POLICY_INPUT"})
	require.NoError(t, err)
	require.Len(t, got, 1)

	assert.Equal(t, "ignored_paths", got[0].Input)
	assert.Equal(t, "ignored_paths", got[0].Column)
	// The env reference is materialized to a real local file.
	assert.NotEqual(t, "env://CHAINLOOP_TEST_POLICY_INPUT", got[0].File)
	content, err := os.ReadFile(got[0].File)
	require.NoError(t, err)
	assert.Equal(t, `["c:\\a.dll"]`, string(content))
}
