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
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/buildlog"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	mUploader "github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Values that other tests of the package repeat.
const (
	buildLogMissingFile   = "no such file or directory"
	buildLogOtherMaterial = "./testdata/sbom-spdx.json"
)

func TestNewBuildLogCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name:  "build log material",
			input: &contractAPI.CraftingSchema_Material{Type: contractAPI.CraftingSchema_Material_BUILD_LOG},
		},
		{
			name:    "other material type",
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

// writeMixedBuildLogZip zips a binary log with a Visual C++ build log, as a
// product that builds with both tools does.
func writeMixedBuildLogZip(t *testing.T) string {
	t.Helper()
	entries := map[string][]byte{}
	for _, n := range []string{"sample.binlog", "vcbuild.htm"} {
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
			name:     "missing file",
			filePath: func(*testing.T) string { return "./testdata/non-existing.binlog" },
			wantErr:  buildLogMissingFile,
		},
		{
			name:     "not a build log",
			filePath: func(*testing.T) string { return buildLogOtherMaterial },
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
				materials.AnnotationToolNameKey:           buildlog.ToolMSBuild,
				materials.AnnotationBuildLogLogsCount:     "1",
				materials.AnnotationBuildLogWarningsCount: "4",
				materials.AnnotationBuildLogErrorsCount:   "0",
			},
		},
		{
			name:     "msbuild text log",
			filePath: func(*testing.T) string { return "./testdata/buildlog/msbuild.txt" },
			annotations: map[string]string{
				materials.AnnotationToolNameKey:           buildlog.ToolMSBuild,
				materials.AnnotationBuildLogWarningsCount: "4",
			},
		},
		{
			name:     "visual c++ build log",
			filePath: func(*testing.T) string { return "./testdata/buildlog/vcbuild.htm" },
			annotations: map[string]string{
				materials.AnnotationToolNameKey:           buildlog.ToolVCBuild,
				materials.AnnotationBuildLogWarningsCount: "4",
			},
		},
		{
			name:     "archive of binary logs",
			filePath: writeBuildLogZip,
			annotations: map[string]string{
				materials.AnnotationToolNameKey:           buildlog.ToolMSBuild,
				materials.AnnotationBuildLogLogsCount:     "2",
				materials.AnnotationBuildLogWarningsCount: "4",
			},
		},
		{
			name:     "archive of binary and Visual C++ build logs",
			filePath: writeMixedBuildLogZip,
			annotations: map[string]string{
				materials.AnnotationToolsKey:          `["msbuild","vcbuild"]`,
				materials.AnnotationBuildLogLogsCount: "2",
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
