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

package crafter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePRMetadataTempFile(t *testing.T) {
	tests := []struct {
		name           string
		prNumber       string
		wantMaterial   string
		wantFilePrefix string
	}{
		{
			name:           "numeric PR number",
			prNumber:       "123",
			wantMaterial:   "pr-metadata-123",
			wantFilePrefix: "pr-metadata-123-",
		},
		{
			name:           "large PR number",
			prNumber:       "99999",
			wantMaterial:   "pr-metadata-99999",
			wantFilePrefix: "pr-metadata-99999-",
		},
		{
			name:           "single digit PR number",
			prNumber:       "1",
			wantMaterial:   "pr-metadata-1",
			wantFilePrefix: "pr-metadata-1-",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			materialName, filePath, err := createPRMetadataTempFile(tc.prNumber, []byte(`{"test": true}`))
			require.NoError(t, err)
			defer os.Remove(filePath)

			assert.Equal(t, tc.wantMaterial, materialName)
			assert.Equal(t, ".json", filepath.Ext(filePath))
			assert.True(t, strings.HasPrefix(filepath.Base(filePath), tc.wantFilePrefix))
		})
	}
}
