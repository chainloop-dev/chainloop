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

package service

import (
	"testing"

	cpAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
)

func TestExtractMaterials(t *testing.T) {
	testCases := []struct {
		name  string
		input []*chainloop.NormalizedMaterial
		want  []*cpAPI.AttestationItem_Material
	}{
		{
			name: "different material types",
			input: []*chainloop.NormalizedMaterial{
				{
					Name:  "foo",
					Type:  "STRING",
					Value: "bar",
				},
				{
					Name:  "with_annotations",
					Type:  "STRING",
					Value: "bar",
					Annotations: map[string]string{
						"foo": "bar",
						"bar": "baz",
					},
				},
				{
					Name:  "foo",
					Type:  "ARTIFACT",
					Value: "bar",
					Hash:  &crv1.Hash{Algorithm: "sha256", Hex: "deadbeef"},
				},
				{
					Name:  "image",
					Type:  "CONTAINER_IMAGE",
					Value: "docker.io/nginx",
					Hash:  &crv1.Hash{Algorithm: "sha256", Hex: "deadbeef"},
				},
			},
			want: []*cpAPI.AttestationItem_Material{
				{
					Name:  "foo",
					Type:  "STRING",
					Value: "bar",
				},
				{
					Name:  "with_annotations",
					Type:  "STRING",
					Value: "bar",
					Annotations: map[string]string{
						"foo": "bar",
						"bar": "baz",
					},
				},
				{
					Name:  "foo",
					Type:  "ARTIFACT",
					Value: "bar",
					Hash:  "sha256:deadbeef",
				},
				{
					Name:  "image",
					Type:  "CONTAINER_IMAGE",
					Value: "docker.io/nginx",
					Hash:  "sha256:deadbeef",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractMaterials(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractEnvVariables(t *testing.T) {
	testCases := []struct {
		name  string
		input map[string]string
		want  []*cpAPI.AttestationItem_EnvVariable
	}{
		{
			name: "returns env vars sorted by name",
			input: map[string]string{
				"Z_VAR": "z",
				"A_VAR": "a",
				"M_VAR": "m",
			},
			want: []*cpAPI.AttestationItem_EnvVariable{
				{Name: "A_VAR", Value: "a"},
				{Name: "M_VAR", Value: "m"},
				{Name: "Z_VAR", Value: "z"},
			},
		},
		{
			name:  "empty input",
			input: map[string]string{},
			want:  []*cpAPI.AttestationItem_EnvVariable{},
		},
		{
			name:  "nil input",
			input: nil,
			want:  []*cpAPI.AttestationItem_EnvVariable{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractEnvVariables(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
