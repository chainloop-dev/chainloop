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
	"context"
	"os"
	"path/filepath"
	"testing"

	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	mUploader "github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// dranzerBundleFiles are the report entries of the reference bundle: one per
// dranzer test mode. checkResult_Dranzer.csv is the companion that is not a report.
var dranzerBundleFiles = []string{
	"example-app_1.0.0_b_Result.txt",
	"example-app_1.0.0_p_Result.txt",
	"example-app_1.0.0_s_Result.txt",
	"example-app_1.0.0_t_Result.txt",
}

// dranzerBundleEntries reads the named files from the reference bundle fixture,
// keyed by the archive entry name they should be stored under.
func dranzerBundleEntries(t *testing.T, names []string) map[string][]byte {
	t.Helper()
	entries := make(map[string][]byte, len(names))
	for _, n := range names {
		content, err := os.ReadFile(filepath.Join("./testdata/dranzer-bundle", n))
		require.NoError(t, err)
		entries["Dranzer/"+n] = content
	}
	return entries
}

// writeDranzerZip builds a zip of the named fixture files, including a directory
// entry as the reference bundle carries one.
func writeDranzerZip(t *testing.T, names []string) string {
	t.Helper()
	entries := dranzerBundleEntries(t, names)
	entries["Dranzer/"] = nil
	p := filepath.Join(t.TempDir(), "Dranzer.zip")
	writeZip(t, p, entries)
	return p
}

// writeDranzerTarGz builds a tar.gz of the named fixture files.
func writeDranzerTarGz(t *testing.T, names []string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "Dranzer.tar.gz")
	writeTarGz(t, p, dranzerBundleEntries(t, names))
	return p
}

func TestNewDranzerCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name:  "happy path",
			input: &contractAPI.CraftingSchema_Material{Type: contractAPI.CraftingSchema_Material_CERTCC_DRANZER},
		},
		{
			name:    "wrong type",
			input:   &contractAPI.CraftingSchema_Material{Type: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := materials.NewDranzerCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestDranzerCrafter_Craft(t *testing.T) {
	testCases := []struct {
		name        string
		filePath    string
		wantErr     string
		annotations map[string]string
	}{
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.txt",
			wantErr:  "no such file or directory",
		},
		{
			name:     "wrong content",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "does not look like dranzer output",
		},
		{
			name:     "valid report",
			filePath: "./testdata/dranzer-report.txt",
			annotations: map[string]string{
				"chainloop.material.tool.name":    "dranzer",
				"chainloop.material.tool.version": "96",
			},
		},
	}

	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_CERTCC_DRANZER,
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
			crafter, err := materials.NewDranzerCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, contractAPI.CraftingSchema_Material_CERTCC_DRANZER.String(), got.MaterialType.String())
			assert.True(t, got.UploadedToCas)
			for k, v := range tc.annotations {
				assert.Equal(t, v, got.Annotations[k])
			}
		})
	}
}

// TestDranzerCrafter_CraftArchive covers the bundle form: one dranzer run emits a
// report per test mode (-b/-p/-s/-t), delivered as a single archive. The archive
// is recorded whole as one material — it is what the customer produced — so it
// fills the contract's declared slot, with the report count as an annotation.
func TestDranzerCrafter_CraftArchive(t *testing.T) {
	schema := &contractAPI.CraftingSchema_Material{
		Name: "dranzer-report",
		Type: contractAPI.CraftingSchema_Material_CERTCC_DRANZER,
	}
	l := zerolog.Nop()

	craft := func(t *testing.T, path string, wantUpload bool) (*api.Attestation_Material, error) {
		t.Helper()
		uploader := mUploader.NewUploader(t)
		if wantUpload {
			uploader.On("Upload", context.TODO(), mock.Anything, mock.Anything, mock.Anything).
				Return(&casclient.UpDownStatus{}, nil)
		}
		crafter, err := materials.NewDranzerCrafter(schema, &casclient.CASBackend{Uploader: uploader}, &l)
		require.NoError(t, err)
		return crafter.Craft(context.TODO(), path)
	}

	t.Run("zip of the four mode reports is accepted", func(t *testing.T) {
		got, err := craft(t, writeDranzerZip(t, dranzerBundleFiles), true)

		require.NoError(t, err)
		assert.Equal(t, "dranzer", got.Annotations["chainloop.material.tool.name"])
		assert.Equal(t, "96", got.Annotations["chainloop.material.tool.version"])
		assert.Equal(t, "4", got.Annotations["chainloop.material.dranzer.reports.count"])
	})

	t.Run("tar.gz of the four mode reports is accepted", func(t *testing.T) {
		got, err := craft(t, writeDranzerTarGz(t, dranzerBundleFiles), true)

		require.NoError(t, err)
		assert.Equal(t, "4", got.Annotations["chainloop.material.dranzer.reports.count"])
	})

	t.Run("the CSV companion alongside the reports does not count as a report", func(t *testing.T) {
		withCSV := append(append([]string{}, dranzerBundleFiles...), "checkResult_Dranzer.csv")

		got, err := craft(t, writeDranzerZip(t, withCSV), true)

		require.NoError(t, err)
		assert.Equal(t, "4", got.Annotations["chainloop.material.dranzer.reports.count"],
			"the CSV must not be counted as a dranzer report")
	})

	t.Run("archive holding only the CSV companion is rejected", func(t *testing.T) {
		_, err := craft(t, writeDranzerZip(t, []string{"checkResult_Dranzer.csv"}), false)

		assert.ErrorIs(t, err, materials.ErrInvalidMaterialType)
	})

	t.Run("archive with no entries at all is rejected", func(t *testing.T) {
		_, err := craft(t, writeDranzerZip(t, nil), false)

		assert.ErrorIs(t, err, materials.ErrInvalidMaterialType)
	})

	t.Run("single report records a count of one", func(t *testing.T) {
		got, err := craft(t, "./testdata/dranzer-bundle/example-app_1.0.0_t_Result.txt", true)

		require.NoError(t, err)
		assert.Equal(t, "1", got.Annotations["chainloop.material.dranzer.reports.count"])
	})
}
