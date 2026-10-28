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

package materials_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	mUploader "github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewBuildLogCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name:  "happy path",
			input: &contractAPI.CraftingSchema_Material{Type: contractAPI.CraftingSchema_Material_BUILD_LOG},
		},
		{
			name:    "wrong type",
			input:   &contractAPI.CraftingSchema_Material{Type: contractAPI.CraftingSchema_Material_SARIF},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := materials.NewBuildLogCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// writeBuildLogZip zips the binary logs of two solutions, the form used when a
// build produces one binary log per solution.
func writeBuildLogZip(t *testing.T) string {
	t.Helper()
	entries := map[string][]byte{}
	for _, n := range []string{"sample.binlog", "lib.binlog"} {
		content, err := os.ReadFile(filepath.Join("./testdata/buildlog", n))
		require.NoError(t, err)
		entries["logs/"+n] = content
	}
	p := filepath.Join(t.TempDir(), "logs.zip")
	writeZip(t, p, entries)
	return p
}

// writeOldBinlog writes a binary log header in a format version that predates
// length-prefixed records.
func writeOldBinlog(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte{14, 0, 0, 0, 14, 0, 0, 0})
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	p := filepath.Join(t.TempDir(), "old.binlog")
	require.NoError(t, os.WriteFile(p, buf.Bytes(), 0o600))
	return p
}

func TestBuildLogCrafter_Craft(t *testing.T) {
	testCases := []struct {
		name        string
		filePath    func(t *testing.T) string
		wantErr     string
		annotations map[string]string
	}{
		{
			name:     "invalid path",
			filePath: func(*testing.T) string { return "./testdata/non-existing.binlog" },
			wantErr:  "no such file or directory",
		},
		{
			name:     "wrong content",
			filePath: func(*testing.T) string { return "./testdata/sbom-spdx.json" },
			wantErr:  "does not look like a build log",
		},
		{
			name:     "unsupported binary log version",
			filePath: writeOldBinlog,
			wantErr:  "unsupported MSBuild binary log version 14",
		},
		{
			name:     "msbuild binary log",
			filePath: func(*testing.T) string { return "./testdata/buildlog/sample.binlog" },
			annotations: map[string]string{
				"chainloop.material.tool.name":                "msbuild",
				"chainloop.material.build_log.logs.count":     "1",
				"chainloop.material.build_log.warnings.count": "4",
				"chainloop.material.build_log.errors.count":   "0",
			},
		},
		{
			name:     "msbuild text log",
			filePath: func(*testing.T) string { return "./testdata/buildlog/msbuild.txt" },
			annotations: map[string]string{
				"chainloop.material.tool.name":                "msbuild",
				"chainloop.material.build_log.warnings.count": "4",
			},
		},
		{
			name:     "visual c++ build log",
			filePath: func(*testing.T) string { return "./testdata/buildlog/vcbuild.htm" },
			annotations: map[string]string{
				"chainloop.material.tool.name":                "vcbuild",
				"chainloop.material.build_log.warnings.count": "4",
			},
		},
		{
			name:     "archive of binary logs",
			filePath: writeBuildLogZip,
			annotations: map[string]string{
				"chainloop.material.tool.name":                "msbuild",
				"chainloop.material.build_log.logs.count":     "2",
				"chainloop.material.build_log.warnings.count": "4",
			},
		},
	}

	schema := &contractAPI.CraftingSchema_Material{
		Name: "build-log",
		Type: contractAPI.CraftingSchema_Material_BUILD_LOG,
	}

	l := zerolog.Nop()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("Upload", context.TODO(), mock.Anything, mock.Anything, mock.Anything).
					Return(&casclient.UpDownStatus{}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewBuildLogCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := craftedMaterial(crafter.Craft(context.TODO(), tc.filePath(t)))
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, contractAPI.CraftingSchema_Material_BUILD_LOG.String(), got.MaterialType.String())
			assert.True(t, got.UploadedToCas)
			for k, v := range tc.annotations {
				assert.Equal(t, v, got.Annotations[k], k)
			}
		})
	}
}
