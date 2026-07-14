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

package trufflehog_test

import (
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/trufflehog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		name         string
		input        string
		wantErr      bool
		wantLen      int
		assertResult func(t *testing.T, findings []trufflehog.Finding)
	}{
		{
			name:    "empty input",
			input:   "",
			wantLen: 0,
		},
		{
			name:    "whitespace only",
			input:   "   \n\n\t\n",
			wantLen: 0,
		},
		{
			name: "two findings, one verified",
			input: `{"SourceMetadata":{"Data":{"Filesystem":{"file":"config.yaml","line":10}}},"SourceID":0,"SourceType":15,"SourceName":"trufflehog - filesystem","DetectorType":8,"DetectorName":"Github","DecoderName":"PLAIN","Verified":true,"Raw":"ghp_secret","Redacted":"ghp_***"}
{"SourceMetadata":{"Data":{"Filesystem":{"file":"app.env","line":3}}},"SourceID":0,"SourceType":15,"SourceName":"trufflehog - filesystem","DetectorType":9,"DetectorName":"AWS","DecoderName":"PLAIN","Verified":false,"Raw":"AKIA...","Redacted":"AKIA***"}`,
			wantLen: 2,
			assertResult: func(t *testing.T, findings []trufflehog.Finding) {
				assert.Equal(t, "Github", findings[0].DetectorName)
				assert.Equal(t, 8, findings[0].DetectorType)
				assert.True(t, findings[0].Verified)
				assert.Equal(t, "ghp_secret", findings[0].Raw)
				assert.Equal(t, "ghp_***", findings[0].Redacted)
				assert.Equal(t, "AWS", findings[1].DetectorName)
				assert.False(t, findings[1].Verified)
				assert.NotEmpty(t, findings[0].SourceMetadata.Data)
			},
		},
		{
			name: "blank lines between findings are skipped",
			input: `{"DetectorName":"Github","Verified":true,"Raw":"x"}

{"DetectorName":"AWS","Verified":false,"Raw":"y"}
`,
			wantLen: 2,
		},
		{
			name:    "empty JSON array (canonical clean scan)",
			input:   `[]`,
			wantLen: 0,
		},
		{
			name:    "empty JSON array with surrounding whitespace",
			input:   "  \n [] \n",
			wantLen: 0,
		},
		{
			name:    "JSON array with findings",
			input:   `[{"DetectorName":"Github","Verified":true,"Raw":"x"},{"DetectorName":"AWS","Verified":false,"Raw":"y"}]`,
			wantLen: 2,
			assertResult: func(t *testing.T, findings []trufflehog.Finding) {
				assert.Equal(t, "Github", findings[0].DetectorName)
				assert.Equal(t, "AWS", findings[1].DetectorName)
			},
		},
		{
			name:    "malformed JSON array",
			input:   `[{"DetectorName":"Github"`,
			wantErr: true,
		},
		{
			// "[]" with findings appended must NOT be read as an empty report;
			// trailing content is rejected so findings cannot be hidden.
			name:    "content appended after empty array is rejected",
			input:   "[]\n{\"DetectorName\":\"Slack\",\"Verified\":true}",
			wantErr: true,
		},
		{
			name:    "content appended after non-empty array is rejected",
			input:   `[{"DetectorName":"Slack"}] {"DetectorName":"AWS"}`,
			wantErr: true,
		},
		{
			name:    "empty array with trailing whitespace is accepted",
			input:   "[]\n  \n",
			wantLen: 0,
		},
		{
			name:    "malformed line",
			input:   `this is not json`,
			wantErr: true,
		},
		{
			name: "valid line followed by malformed line",
			input: `{"DetectorName":"Github","Verified":true}
{invalid}`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := trufflehog.Parse(strings.NewReader(tc.input))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, findings, tc.wantLen)
			if tc.assertResult != nil {
				tc.assertResult(t, findings)
			}
		})
	}
}
