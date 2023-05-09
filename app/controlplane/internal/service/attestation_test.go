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

package service

import (
	"testing"

	cpAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
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
					Name:  "foo",
					Type:  "ARTIFACT",
					Value: "bar",
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
					Name:  "foo",
					Type:  "ARTIFACT",
					Value: "bar@sha256:deadbeef",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractMaterials(tc.input)
			assert.Equal(t, got, tc.want)
		})
	}

}
