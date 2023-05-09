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

	api "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestRenderV01(t *testing.T) {
	testCases := []struct {
		name       string
		sourcePath string
		outputPath string
	}{
		{
			name:       "render v0.1",
			sourcePath: "testdata/attestation.source.json",
			outputPath: "testdata/attestation.output.v0.1.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load expected resulting output
			wantRaw, err := os.ReadFile(tc.outputPath)
			require.NoError(t, err)

			var want *in_toto.Statement
			err = json.Unmarshal(wantRaw, &want)
			require.NoError(t, err)

			// Initialize renderer
			state := &api.CraftingState{}
			stateRaw, err := os.ReadFile(tc.sourcePath)
			require.NoError(t, err)

			err = protojson.Unmarshal(stateRaw, state)
			require.NoError(t, err)

			renderer := NewChainloopRendererV01(state.Attestation, "dev", "sha256:59e14f1a9de709cdd0e91c36b33e54fcca95f7dba1dc7169a7f81986e02108e5")

			// Compare header
			gotHeader, err := renderer.Header()
			assert.NoError(t, err)
			assert.Equal(t, want.Type, gotHeader.Type)
			assert.Equal(t, want.Subject, gotHeader.Subject)
			assert.Equal(t, want.PredicateType, gotHeader.PredicateType)

			// Compare predicate
			gotPredicateI, err := renderer.Predicate()
			assert.NoError(t, err)
			gotPredicate := gotPredicateI.(ProvenancePredicateV01)

			wantPredicate := ProvenancePredicateV01{}
			err = extractPredicate(want, &wantPredicate)
			assert.NoError(t, err)
			wantPredicate.Metadata.FinishedAt = gotPredicate.Metadata.FinishedAt
			assert.EqualValues(t, wantPredicate, gotPredicate)
		})
	}
}

func TestExtractPredicate(t *testing.T) {
	testCases := []struct {
		name         string
		envelopePath string
		envVars      map[string]string
		materials    []*NormalizedMaterial
		wantErr      bool
	}{
		{
			name:         "valid envelope",
			envelopePath: "testdata/valid.envelope.json",
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
