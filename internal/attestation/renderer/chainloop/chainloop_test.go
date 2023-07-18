//
// Copyright 2023 The Chainloop Authors.
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

package chainloop

import (
	"encoding/json"
	"os"
	"testing"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPredicate(t *testing.T) {
	testCases := []struct {
		name         string
		envelopePath string
		envVars      map[string]string
		materials    []*NormalizedMaterial
		wantErr      bool
	}{
		{
			name:         "valid envelope v1",
			envelopePath: "testdata/valid.envelope.v1.json",
			envVars:      map[string]string{"GITHUB_ACTOR": "migmartri", "GITHUB_REF": "refs/tags/v0.0.39", "GITHUB_REPOSITORY": "chainloop-dev/integration-demo", "GITHUB_REPOSITORY_OWNER": "chainloop-dev", "GITHUB_RUN_ID": "4410543365", "GITHUB_SHA": "0accc9392fb1f9b258167c18ffa0aeb626973f1c", "RUNNER_NAME": "Hosted Agent", "RUNNER_OS": "Linux"},
			materials: []*NormalizedMaterial{
				{
					Name: "binary", Type: "ARTIFACT",
					Value: "integration-demo_0.0.39_linux_amd64.tar.gz",
					Hash:  &crv1.Hash{Algorithm: "sha256", Hex: "b155cdfc328b273c4b741c08b3b84ac441b0562ca51893f23495b35abf89ea87"},
				},
				{
					Name: "image", Type: "CONTAINER_IMAGE",
					Value: "ghcr.io/chainloop-dev/integration-demo",
					Hash:  &crv1.Hash{Algorithm: "sha256", Hex: "e0d8179991dd735baf0961901b33476a76a0f300bc4ea07e3d7ae7c24e147193"},
				},
				{
					Name: "sbom", Type: "SBOM_CYCLONEDX_JSON",
					Value: "sbom.cyclonedx.json",
					Hash:  &crv1.Hash{Algorithm: "sha256", Hex: "b50f38961cc2e97d0903f4683a40e2528f7f6c9d382e8c6048b0363af95b7080"},
				},
			},
		},
		{
			name:         "valid envelope v2",
			envelopePath: "testdata/valid.envelope.v2.json",
			envVars:      map[string]string{"CUSTOM_VAR": "foobar"},
			materials: []*NormalizedMaterial{
				{
					Name: "binary", Type: "ARTIFACT",
					Filename:      "main.go",
					Hash:          &crv1.Hash{Algorithm: "sha256", Hex: "8fce0203a4efaac3b08ee3ad769233039faa762a3da0777c45b315f398f0c150"},
					UploadedToCAS: true,
				},
				{
					Name: "image", Type: "CONTAINER_IMAGE",
					Value: "index.docker.io/bitnami/nginx",
					Hash:  &crv1.Hash{Algorithm: "sha256", Hex: "747ef335ea27a2faf08aa292a5bc5491aff50c6a94ee4ebcbbcd43cdeccccaf1"},
				},
				{
					Name: "sbom", Type: "SBOM_CYCLONEDX_JSON",
					Filename:      "sbom.cyclonedx.json",
					Hash:          &crv1.Hash{Algorithm: "sha256", Hex: "16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c"},
					UploadedToCAS: true,
				},
				{
					Name: "sbom", Type: "SBOM_CYCLONEDX_JSON",
					Filename:       "inline-sbom.json",
					Hash:           &crv1.Hash{Algorithm: "sha256", Hex: "16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c"},
					EmbeddedInline: true,
					Value:          "hello inline!",
				},
				{
					Name: "stringvar", Type: "STRING",
					Value: "helloworld",
				},
			},
		},
		{
			name:         "unknown source attestation",
			envelopePath: "testdata/unknown.envelope.json",
			wantErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			envelope, err := testEnvelope(tc.envelopePath)
			require.NoError(t, err)

			gotPredicate, err := ExtractPredicate(envelope)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			gotVars := gotPredicate.GetEnvVars()
			assert.Equal(t, tc.envVars, gotVars)

			gotMaterials := gotPredicate.GetMaterials()
			assert.Equal(t, tc.materials, gotMaterials)
		})
	}
}

func testEnvelope(filePath string) (*dsse.Envelope, error) {
	var envelope dsse.Envelope
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(content, &envelope)
	if err != nil {
		return nil, err
	}

	return &envelope, nil
}
