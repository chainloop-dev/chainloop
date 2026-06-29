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

package action

import (
	"regexp"
	"testing"

	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/stretchr/testify/assert"
)

// materialNameRe mirrors the DNS-1123-style constraint enforced on material
// names by the proto validation (name.dns-1123).
var materialNameRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func TestPolicyInputEvidenceNames(t *testing.T) {
	testCases := []struct {
		name         string
		materialName string
		inputs       []string
		want         []string
	}{
		{
			name:         "underscores in the input become hyphens",
			materialName: "sigcheck",
			inputs:       []string{"ignored_paths"},
			want:         []string{"sigcheck-ignored-paths"},
		},
		{
			name:         "distinct inputs keep their (sanitized) names",
			materialName: "sigcheck",
			inputs:       []string{"ignored_paths", "third_party_paths"},
			want:         []string{"sigcheck-ignored-paths", "sigcheck-third-party-paths"},
		},
		{
			name:         "same input fed by multiple files is disambiguated",
			materialName: "sigcheck",
			inputs:       []string{"ignored_paths", "ignored_paths"},
			want:         []string{"sigcheck-ignored-paths-1", "sigcheck-ignored-paths-2"},
		},
		{
			name:         "uppercase and odd characters are normalized",
			materialName: "binaries",
			inputs:       []string{"Ignored Paths!!"},
			want:         []string{"binaries-ignored-paths"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			files := make([]*PolicyInputFromFile, len(tc.inputs))
			for i, in := range tc.inputs {
				files[i] = &PolicyInputFromFile{Input: in}
			}
			got := policyInputEvidenceNames(tc.materialName, files)
			assert.Equal(t, tc.want, got)
			for _, n := range got {
				assert.Regexp(t, materialNameRe, n, "generated material name must be a valid DNS-1123 name")
			}
		})
	}
}

func TestSanitizeMaterialNamePartIsValid(t *testing.T) {
	// Even adversarial inputs must yield a part that, joined onto a material
	// name, stays a valid DNS-1123 name.
	for _, in := range []string{"ignored_paths", "  spaced  ", "UPPER", "a..b__c", "!!!", "", "_leading", "trailing_", "mixed/sep\\chars"} {
		part := sanitizeMaterialNamePart(in)
		assert.Regexp(t, materialNameRe, "m-"+part, "input %q -> part %q", in, part)
	}
}

func TestAddReference(t *testing.T) {
	t.Run("sets references on a material with no annotations", func(t *testing.T) {
		m := &api.Attestation_Material{}
		addReference(m, "sigcheck-ignored-paths", "sigcheck-third-party-paths")
		assert.Equal(t, "sigcheck-ignored-paths,sigcheck-third-party-paths", m.Annotations[materials.AnnotationMaterialReferences])
	})

	t.Run("appends to and de-duplicates existing references", func(t *testing.T) {
		m := &api.Attestation_Material{Annotations: map[string]string{
			materials.AnnotationMaterialReferences: "existing",
		}}
		addReference(m, "existing", "new")
		assert.Equal(t, "existing,new", m.Annotations[materials.AnnotationMaterialReferences])
	})

	t.Run("no names is a no-op", func(t *testing.T) {
		m := &api.Attestation_Material{}
		addReference(m)
		assert.Empty(t, m.Annotations[materials.AnnotationMaterialReferences])
	})
}

func TestBuildRuntimeInputsNil(t *testing.T) {
	got, err := buildRuntimeInputs(nil)
	assert.NoError(t, err)
	assert.Nil(t, got)
}
