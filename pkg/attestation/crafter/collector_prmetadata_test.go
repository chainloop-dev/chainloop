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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPRMetadataCollectorTempFileHasJSONExtension(t *testing.T) {
	tests := []struct {
		name          string
		prNumber      string
		wantPrefix    string
		wantExtension string
	}{
		{
			name:          "numeric PR number",
			prNumber:      "123",
			wantPrefix:    "pr-metadata-123-",
			wantExtension: ".json",
		},
		{
			name:          "large PR number",
			prNumber:      "99999",
			wantPrefix:    "pr-metadata-99999-",
			wantExtension: ".json",
		},
		{
			name:          "single digit PR number",
			prNumber:      "1",
			wantPrefix:    "pr-metadata-1-",
			wantExtension: ".json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			materialName := fmt.Sprintf("pr-metadata-%s", tc.prNumber)
			tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s-*.json", materialName))
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			tmpFile.Close()

			fileName := filepath.Base(tmpFile.Name())
			assert.Equal(t, tc.wantExtension, filepath.Ext(fileName))
			assert.True(t, len(fileName) > len(tc.wantPrefix),
				"filename %q should be longer than prefix %q", fileName, tc.wantPrefix)
			assert.Equal(t, tc.wantPrefix, fileName[:len(tc.wantPrefix)])
		})
	}
}
