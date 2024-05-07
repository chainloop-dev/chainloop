//
// Copyright 2024 The Chainloop Authors.
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

package schemavalidators_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	"github.com/stretchr/testify/require"
)

func TestValidateCycloneDX1_5(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "invalid sbom format",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "missing properties: 'bomFormat', 'specVersion'",
		},
		{
			name:     "invalid clycondx format",
			filePath: "./testdata/openvex_v0.2.0.json",
			wantErr:  "missing properties: 'bomFormat', 'specVersion'",
		},
		{
			name:     "1.4 version",
			filePath: "./testdata/sbom.cyclonedx.json",
		},
		{
			name:     "1.5 version",
			filePath: "./testdata/sbom.cyclonedx-1.5.json",
		},
		{
			name:     "1.6 version",
			filePath: "./testdata/sbom.cyclonedx-1.6.json",
			wantErr:  "value must be one of \"application\", \"framework\", \"library\",",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.ReadFile(tc.filePath)
			require.NoError(t, err)

			var v interface{}
			require.NoError(t, json.Unmarshal(f, &v))

			err = schemavalidators.ValidateCycloneDX(v, "1.5")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestValidateCycloneDX1_6(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "invalid sbom format",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  " missing properties: 'bomFormat', 'specVersion'",
		},
		{
			name:     "invalid clycondx format",
			filePath: "./testdata/openvex_v0.2.0.json",
			wantErr:  "missing properties: 'bomFormat', 'specVersion'",
		},
		{
			name:     "1.4 version",
			filePath: "./testdata/sbom.cyclonedx.json",
		},
		{
			name:     "1.5 version",
			filePath: "./testdata/sbom.cyclonedx-1.5.json",
		},
		{
			name:     "1.6 version",
			filePath: "./testdata/sbom.cyclonedx-1.6.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.ReadFile(tc.filePath)
			require.NoError(t, err)

			var v interface{}
			require.NoError(t, json.Unmarshal(f, &v))

			err = schemavalidators.ValidateCycloneDX(v, "1.6")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}
