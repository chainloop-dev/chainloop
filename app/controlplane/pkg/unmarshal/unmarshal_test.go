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

package unmarshal

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentifyFormat(t *testing.T) {
	testData := []struct {
		filename   string
		wantFormat RawFormat
		wantErr    bool
	}{
		{
			filename:   "contract.cue",
			wantFormat: RawFormatCUE,
		},
		{
			filename:   "contract.json",
			wantFormat: RawFormatJSON,
		},
		{
			filename:   "invalid_contract.json",
			wantFormat: RawFormatJSON,
		},
		{
			filename:   "contract.yaml",
			wantFormat: RawFormatYAML,
		},
		{
			filename:   "invalid_contract.yaml",
			wantFormat: RawFormatYAML,
		},
		{
			filename: "invalid_format.json",
			wantErr:  true,
		},
	}

	for _, tt := range testData {
		t.Run(tt.filename, func(t *testing.T) {
			// load file from testdata/contracts
			data, err := os.ReadFile("testdata/contracts/" + tt.filename)
			require.NoError(t, err)

			format, err := IdentifyFormat(data)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tt.wantFormat, format)
		})
	}
}
