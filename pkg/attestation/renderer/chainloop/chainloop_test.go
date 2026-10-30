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

package chainloop

import (
	"encoding/json"
	"os"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
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
			name:         "valid envelope v2",
			envelopePath: "testdata/valid.envelope.v2.json",
			envVars:      map[string]string{"CUSTOM_VAR": "foobar"},
			materials: []*NormalizedMaterial{
				{
					Name: "binary", Type: "ARTIFACT",
					Filename:      "main.go",
					Hash:          &crv1.Hash{Algorithm: "sha256", Hex: "8fce0203a4efaac3b08ee3ad769233039faa762a3da0777c45b315f398f0c150"},
					UploadedToCAS: true,
					Annotations: map[string]string{
						"annotation":              "baz",
						"chainloop.material.cas":  "true",
						"chainloop.material.name": "binary",
						"chainloop.material.type": "ARTIFACT",
					},
				},
				{
					Name: "image", Type: "CONTAINER_IMAGE",
					Value: []byte("index.docker.io/bitnami/nginx"),
					Hash:  &crv1.Hash{Algorithm: "sha256", Hex: "747ef335ea27a2faf08aa292a5bc5491aff50c6a94ee4ebcbbcd43cdeccccaf1"},
					Annotations: map[string]string{
						"another_annotation":      "foo",
						"chainloop.material.name": "image",
						"chainloop.material.type": "CONTAINER_IMAGE",
					},
				},
				{
					Name: "sbom", Type: "SBOM_CYCLONEDX_JSON",
					Filename:      "sbom.cyclonedx.json",
					Hash:          &crv1.Hash{Algorithm: "sha256", Hex: "16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c"},
					UploadedToCAS: true,
					Annotations: map[string]string{
						"chainloop.material.cas":  "true",
						"chainloop.material.name": "sbom",
						"chainloop.material.type": "SBOM_CYCLONEDX_JSON",
					},
				},
				{
					Name: "sbom", Type: "SBOM_CYCLONEDX_JSON",
					Filename:       "inline-sbom.json",
					Hash:           &crv1.Hash{Algorithm: "sha256", Hex: "16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c"},
					EmbeddedInline: true,
					Value:          []byte("hello inline!"),
					Annotations: map[string]string{
						"chainloop.material.cas.inline": "true",
						"chainloop.material.name":       "sbom",
						"chainloop.material.type":       "SBOM_CYCLONEDX_JSON",
					},
				},
				{
					Name: "stringvar", Type: "STRING",
					Value: []byte("helloworld"),
					Annotations: map[string]string{
						"chainloop.material.name": "stringvar",
						"chainloop.material.type": "STRING",
					},
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

			require.NoError(t, err)
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

func TestPredicateCommonAuth(t *testing.T) {
	testCases := []struct {
		name     string
		auth     *v1.Attestation_Auth
		wantJSON string
	}{
		{
			name:     "no auth",
			wantJSON: `null`,
		},
		{
			name:     "user token",
			auth:     &v1.Attestation_Auth{Type: v1.Attestation_Auth_AUTH_TYPE_USER, Id: "user-id"},
			wantJSON: `{"id":"user-id","type":"AUTH_TYPE_USER"}`,
		},
		{
			name:     "api token with name",
			auth:     &v1.Attestation_Auth{Type: v1.Attestation_Auth_AUTH_TYPE_API_TOKEN, Id: "token-id", Name: "release-pipeline-token"},
			wantJSON: `{"id":"token-id","type":"AUTH_TYPE_API_TOKEN","name":"release-pipeline-token"}`,
		},
		{
			name:     "api token without name",
			auth:     &v1.Attestation_Auth{Type: v1.Attestation_Auth_AUTH_TYPE_API_TOKEN, Id: "token-id"},
			wantJSON: `{"id":"token-id","type":"AUTH_TYPE_API_TOKEN"}`,
		},
		{
			name:     "federated token with subject",
			auth:     &v1.Attestation_Auth{Type: v1.Attestation_Auth_AUTH_TYPE_FEDERATED, Id: "https://token.actions.githubusercontent.com", Subject: "repo:chainloop-dev/chainloop:ref:refs/heads/main"},
			wantJSON: `{"id":"https://token.actions.githubusercontent.com","type":"AUTH_TYPE_FEDERATED","subject":"repo:chainloop-dev/chainloop:ref:refs/heads/main"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := predicateCommon(&builderInfo{}, &v1.Attestation{Auth: tc.auth})
			raw, err := json.Marshal(got.Auth)
			require.NoError(t, err)
			assert.JSONEq(t, tc.wantJSON, string(raw))
		})
	}
}
