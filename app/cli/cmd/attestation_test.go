//
// Copyright 2023-2026 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestOrgFromLocalState(t *testing.T) {
	t.Run("returns org from valid state file", func(t *testing.T) {
		state := &crafter.VersionedCraftingState{
			CraftingState: &v1.CraftingState{
				Attestation: &v1.Attestation{
					Workflow: &v1.WorkflowMetadata{
						Organization: "my-org",
					},
				},
			},
		}

		raw, err := protojson.Marshal(state)
		require.NoError(t, err)

		statePath := filepath.Join(t.TempDir(), "state.json")
		require.NoError(t, os.WriteFile(statePath, raw, 0o600))

		assert.Equal(t, "my-org", orgFromLocalState(statePath))
	})

	t.Run("returns empty for missing file", func(t *testing.T) {
		assert.Empty(t, orgFromLocalState(filepath.Join(t.TempDir(), "nonexistent.json")))
	})

	t.Run("returns empty for invalid json", func(t *testing.T) {
		statePath := filepath.Join(t.TempDir(), "bad.json")
		require.NoError(t, os.WriteFile(statePath, []byte("not json"), 0o600))
		assert.Empty(t, orgFromLocalState(statePath))
	})

	t.Run("returns empty when org not set in state", func(t *testing.T) {
		state := &crafter.VersionedCraftingState{
			CraftingState: &v1.CraftingState{
				Attestation: &v1.Attestation{
					Workflow: &v1.WorkflowMetadata{},
				},
			},
		}

		raw, err := protojson.Marshal(state)
		require.NoError(t, err)

		statePath := filepath.Join(t.TempDir(), "state.json")
		require.NoError(t, os.WriteFile(statePath, raw, 0o600))

		assert.Empty(t, orgFromLocalState(statePath))
	})
}

func TestAttestationInitValidatesFlagsRegardlessOfRepositoryConfig(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{name: "missing"},
		{name: "unreadable", setup: func(t *testing.T, dir string) {
			require.NoError(t, os.Mkdir(filepath.Join(dir, ".chainloop.yml"), 0o700))
		}},
		{name: "present", setup: func(t *testing.T, dir string) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("{}\n"), 0o600))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.setup != nil {
				tc.setup(t, dir)
			}
			t.Chdir(dir)

			cmd := newAttestationInitCmd()
			require.NoError(t, cmd.Flags().Set("workflow", "build"))
			require.NoError(t, cmd.Flags().Set("release", "true"))

			assert.EqualError(t, cmd.PreRunE(cmd, nil), "project version is required when using --release")
		})
	}
}

func TestExtractAnnotations(t *testing.T) {
	testCases := []struct {
		input   []string
		want    map[string]string
		wantErr bool
	}{
		{
			input: []string{
				"foo=bar",
				"baz=qux",
			},
			want: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
			wantErr: false,
		},
		{
			input: []string{
				"foo=bar",
				"baz",
			},
			wantErr: true,
		},
		{
			input: []string{
				"foo=bar",
				"baz=qux",
				"foo=bar",
			},
			want: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
			wantErr: false,
		},
		{
			input: []string{
				"foo=bar",
				"baz=qux=qux",
			},
			want: map[string]string{
				"foo": "bar",
				"baz": "qux=qux",
			},
			wantErr: false,
		},
		{
			input: []string{
				"url=https://example.com/path?id=123&foo=bar",
			},
			want: map[string]string{
				"url": "https://example.com/path?id=123&foo=bar",
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		got, err := extractAnnotations(tc.input)
		if tc.wantErr {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)
		assert.Equal(t, tc.want, got)
	}
}
