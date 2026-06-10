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

package sigcheck_test

import (
	"encoding/json"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/sigcheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	colPath     = "Path"
	colVerified = "Verified"
	pathADLL    = "c:\\a.dll"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		name       string
		raw        []byte
		wantErr    bool
		wantHeader []string
		wantRows   int
		wantFirst  map[string]string // subset asserted on rows[0]
	}{
		{
			name:       "comma csv",
			raw:        []byte("\"Path\",\"Verified\"\n\"c:\\a.dll\",\"Signed\"\n\"c:\\b.exe\",\"Unsigned\"\n"),
			wantHeader: []string{colPath, colVerified},
			wantRows:   2,
			wantFirst:  map[string]string{colPath: pathADLL, colVerified: "Signed"},
		},
		{
			name:       "tab separated",
			raw:        []byte("Path\tVerified\tCompany\nc:\\a.dll\tSigned\tMicrosoft\n"),
			wantHeader: []string{colPath, colVerified, "Company"},
			wantRows:   1,
			wantFirst:  map[string]string{colPath: pathADLL, "Company": "Microsoft"},
		},
		{
			name:       "utf-8 BOM stripped",
			raw:        append([]byte{0xEF, 0xBB, 0xBF}, []byte("Path,Verified\nc:\\a.dll,Signed\n")...),
			wantHeader: []string{colPath, colVerified},
			wantRows:   1,
			wantFirst:  map[string]string{colPath: pathADLL},
		},
		{
			name:       "utf-16 LE BOM decoded",
			raw:        utf16LE("Path,Verified\nc:\\a.dll,Signed\n"),
			wantHeader: []string{colPath, colVerified},
			wantRows:   1,
			wantFirst:  map[string]string{colPath: pathADLL},
		},
		{
			name:       "utf-16 BE BOM decoded",
			raw:        utf16BE("Path,Verified\nc:\\a.dll,Signed\n"),
			wantHeader: []string{colPath, colVerified},
			wantRows:   1,
			wantFirst:  map[string]string{colPath: pathADLL},
		},
		{
			name:       "header only is a clean scan",
			raw:        []byte("Path,Verified\n"),
			wantHeader: []string{colPath, colVerified},
			wantRows:   0,
		},
		{
			name:    "empty input",
			raw:     []byte("   \n"),
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := sigcheck.Parse(tc.raw)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantHeader, report.Header)
			assert.Len(t, report.Rows, tc.wantRows)
			for k, v := range tc.wantFirst {
				assert.Equal(t, v, report.Rows[0][k])
			}
		})
	}
}

func TestReportJSON(t *testing.T) {
	report, err := sigcheck.Parse([]byte("Path,Verified\nc:\\a.dll,Signed\n"))
	require.NoError(t, err)

	out, err := report.JSON()
	require.NoError(t, err)

	var rows []map[string]string
	require.NoError(t, json.Unmarshal(out, &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "Signed", rows[0][colVerified])
}

func TestReportJSONEmptyIsArray(t *testing.T) {
	report, err := sigcheck.Parse([]byte("Path,Verified\n"))
	require.NoError(t, err)

	out, err := report.JSON()
	require.NoError(t, err)
	assert.Equal(t, "[]", string(out))
}

func TestHasColumns(t *testing.T) {
	report, err := sigcheck.Parse([]byte("Path,Verified,Company\nc:\\a.dll,Signed,MS\n"))
	require.NoError(t, err)

	assert.True(t, report.HasColumns(colPath, colVerified))
	assert.False(t, report.HasColumns(colPath, "Nonexistent"))
}

// utf16LE encodes s as UTF-16 little-endian with a BOM, mimicking PowerShell redirection.
func utf16LE(s string) []byte {
	out := make([]byte, 0, 2+2*len(s))
	out = append(out, 0xFF, 0xFE) // LE BOM
	for _, r := range s {
		out = append(out, byte(r), byte(r>>8))
	}
	return out
}

// utf16BE encodes s as UTF-16 big-endian with a BOM.
func utf16BE(s string) []byte {
	out := make([]byte, 0, 2+2*len(s))
	out = append(out, 0xFE, 0xFF) // BE BOM
	for _, r := range s {
		out = append(out, byte(r>>8), byte(r))
	}
	return out
}
