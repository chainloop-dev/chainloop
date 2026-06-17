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
	"archive/tar"
	"archive/zip"
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

func TestNewRadamsaReportCrafter(t *testing.T) {
	tests := []struct {
		name    string
		kind    contractAPI.CraftingSchema_Material_MaterialType
		wantErr bool
	}{
		{name: "happy path", kind: contractAPI.CraftingSchema_Material_RADAMSA_REPORT},
		{name: "wrong type", kind: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := materials.NewRadamsaReportCrafter(&contractAPI.CraftingSchema_Material{Type: tc.kind}, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestRadamsaReportCrafter_Craft(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{name: "invalid path", filePath: "./testdata/nope.log", wantErr: "no such file"},
		{name: "not a meta log", filePath: "./testdata/radamsa-meta-invalid.txt", wantErr: "invalid radamsa -M metadata log"},
		{name: "valid -M log", filePath: "./testdata/radamsa-meta.txt"},
	}
	schema := &contractAPI.CraftingSchema_Material{Name: "report", Type: contractAPI.CraftingSchema_Material_RADAMSA_REPORT}
	l := zerolog.Nop()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("Upload", context.TODO(), mock.Anything, mock.Anything, mock.Anything).
					Return(&casclient.UpDownStatus{}, nil)
			}
			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewRadamsaReportCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, contractAPI.CraftingSchema_Material_RADAMSA_REPORT.String(), got.MaterialType.String())
			assert.Equal(t, "radamsa", got.Annotations["chainloop.material.tool.name"])
			assert.True(t, got.UploadedToCas)
		})
	}
}

func TestNewRadamsaCrashesCrafter(t *testing.T) {
	_, err := materials.NewRadamsaCrashesCrafter(&contractAPI.CraftingSchema_Material{Type: contractAPI.CraftingSchema_Material_RADAMSA_CRASHES}, nil, nil)
	assert.NoError(t, err)
	_, err = materials.NewRadamsaCrashesCrafter(&contractAPI.CraftingSchema_Material{Type: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE}, nil, nil)
	assert.Error(t, err)
}

func TestRadamsaCrashesCrafter_Craft(t *testing.T) {
	dir := t.TempDir()
	emptyTar := filepath.Join(dir, "crashes-empty.tar.gz")
	writeTarGz(t, emptyTar, nil)
	twoTar := filepath.Join(dir, "crashes.tar.gz")
	writeTarGz(t, twoTar, map[string][]byte{"c_1.eds": []byte("AAAA"), "c_2.eds": []byte("BBBB")})
	emptyZip := filepath.Join(dir, "crashes-empty.zip")
	writeZip(t, emptyZip, nil)
	twoZip := filepath.Join(dir, "crashes.zip")
	writeZip(t, twoZip, map[string][]byte{"c_1.eds": []byte("AAAA"), "c_2.eds": []byte("BBBB")})
	singleFile := filepath.Join(dir, "c_7.eds")
	require.NoError(t, os.WriteFile(singleFile, []byte("crashing bytes"), 0o600))
	emptyFile := filepath.Join(dir, "empty.bin")
	require.NoError(t, os.WriteFile(emptyFile, nil, 0o600))

	tests := []struct {
		name      string
		filePath  string
		wantErr   string
		wantCount string
	}{
		{name: "empty tar.gz archive => count 0", filePath: emptyTar, wantCount: "0"},
		{name: "tar.gz with two crashes => count 2", filePath: twoTar, wantCount: "2"},
		{name: "empty zip archive => count 0", filePath: emptyZip, wantCount: "0"},
		{name: "zip with two crashes => count 2", filePath: twoZip, wantCount: "2"},
		{name: "single crash file => count 1", filePath: singleFile, wantCount: "1"},
		{name: "empty single file", filePath: emptyFile, wantErr: "empty"},
		{name: "missing file", filePath: filepath.Join(dir, "nope"), wantErr: "no such file"},
	}
	schema := &contractAPI.CraftingSchema_Material{Name: "crashes", Type: contractAPI.CraftingSchema_Material_RADAMSA_CRASHES}
	l := zerolog.Nop()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("Upload", context.TODO(), mock.Anything, mock.Anything, mock.Anything).
					Return(&casclient.UpDownStatus{}, nil)
			}
			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewRadamsaCrashesCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "radamsa", got.Annotations["chainloop.material.tool.name"])
			assert.Equal(t, tc.wantCount, got.Annotations["chainloop.material.radamsa.crashes.count"])
		})
	}
}

// writeTarGz writes a .tar.gz with the given files (name->content); nil => empty archive.
func writeTarGz(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content)), Typeflag: tar.TypeReg}))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
}

// writeZip writes a .zip with the given files (name->content); nil => empty archive.
func writeZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
}
