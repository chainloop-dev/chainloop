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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPolicyInputEvidenceNames(t *testing.T) {
	testCases := []struct {
		name         string
		materialName string
		files        []*PolicyInputFromFile
		want         []string
	}{
		{
			name:         "single input keeps the plain name",
			materialName: "binaries",
			files:        []*PolicyInputFromFile{{Input: "ignored_paths"}},
			want:         []string{"binaries-ignored_paths"},
		},
		{
			name:         "distinct inputs are not suffixed",
			materialName: "binaries",
			files:        []*PolicyInputFromFile{{Input: "ignored_paths"}, {Input: "paths"}},
			want:         []string{"binaries-ignored_paths", "binaries-paths"},
		},
		{
			name:         "same input fed by multiple files is disambiguated",
			materialName: "binaries",
			files:        []*PolicyInputFromFile{{Input: "ignored_paths"}, {Input: "ignored_paths"}},
			want:         []string{"binaries-ignored_paths-1", "binaries-ignored_paths-2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := policyInputEvidenceNames(tc.materialName, tc.files)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBuildRuntimeInputsNil(t *testing.T) {
	got, err := buildRuntimeInputs(nil)
	assert.NoError(t, err)
	assert.Nil(t, got)
}
