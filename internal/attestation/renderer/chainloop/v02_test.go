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
	slsa_v1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestRenderV02(t *testing.T) {
	testCases := []struct {
		name       string
		sourcePath string
		outputPath string
	}{
		{
			name:       "render v0.2",
			sourcePath: "testdata/attestation.source.json",
			outputPath: "testdata/attestation.output.v0.2.json",
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

			renderer := NewChainloopRendererV02(state.Attestation, "dev", "sha256:59e14f1a9de709cdd0e91c36b33e54fcca95f7dba1dc7169a7f81986e02108e5")

			// Compare header
			gotHeader, err := renderer.Header()
			assert.NoError(t, err)
			assert.Equal(t, want.Type, gotHeader.Type)
			assert.Equal(t, want.Subject, gotHeader.Subject)
			assert.Equal(t, want.PredicateType, gotHeader.PredicateType)

			// Compare predicate
			gotPredicateI, err := renderer.Predicate()
			assert.NoError(t, err)
			gotPredicate := gotPredicateI.(ProvenancePredicateV02)

			wantPredicate := ProvenancePredicateV02{}
			err = extractPredicate(want, &wantPredicate)
			assert.NoError(t, err)
			wantPredicate.Metadata.FinishedAt = gotPredicate.Metadata.FinishedAt
			assert.EqualValues(t, wantPredicate, gotPredicate)
		})
	}
}

func TestNormalizeMaterial(t *testing.T) {
	testCases := []struct {
		name    string
		input   *slsa_v1.ResourceDescriptor
		want    *NormalizedMaterial
		wantErr bool
	}{
		{
			name: "invalid material type",
			input: &slsa_v1.ResourceDescriptor{
				Annotations: map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "INVALID",
				},
			},
			wantErr: true,
		},
		{
			name: "missing material type",
			input: &slsa_v1.ResourceDescriptor{
				Annotations: map[string]interface{}{
					"chainloop.material.name": "foo",
				},
			},
			wantErr: true,
		},
		{
			name: "missing material name",
			input: &slsa_v1.ResourceDescriptor{
				Annotations: map[string]interface{}{
					"chainloop.material.type": "STRING",
				},
			},
			wantErr: true,
		},
		{
			name: "valid string material",
			input: &slsa_v1.ResourceDescriptor{
				Annotations: map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "STRING",
				},
				Content: []byte("bar"),
			},
			want: &NormalizedMaterial{
				Name:  "foo",
				Type:  "STRING",
				Value: "bar",
			},
		},
		{
			name: "empty string material",
			input: &slsa_v1.ResourceDescriptor{
				Annotations: map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "STRING",
				},
			},
			wantErr: true,
		},
		{
			name: "valid artifact material",
			input: &slsa_v1.ResourceDescriptor{
				Annotations: map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "ARTIFACT",
					"chainloop.material.cas":  true,
				},
				Digest: map[string]string{
					"sha256": "deadbeef",
				},
				Name: "artifact.tgz",
			},
			want: &NormalizedMaterial{
				Name:                "foo",
				Type:                "ARTIFACT",
				Value:               "artifact.tgz",
				Hash:                &crv1.Hash{Algorithm: "sha256", Hex: "deadbeef"},
				DownloadableFromCAS: true,
			},
		},
		{
			name: "invalid artifact material, missing file name",
			input: &slsa_v1.ResourceDescriptor{
				Annotations: map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "ARTIFACT",
				},
				Digest: map[string]string{
					"sha256": "deadbeef",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid artifact material, missing digest",
			input: &slsa_v1.ResourceDescriptor{
				Annotations: map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "ARTIFACT",
				},
				Name: "artifact.tgz",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeMaterial(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, tc.want, got)
			}
		})
	}
}
