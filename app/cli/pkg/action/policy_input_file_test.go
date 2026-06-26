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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePolicyInputFromFile(t *testing.T) {
	testCases := []struct {
		name    string
		raw     string
		want    *PolicyInputFromFile
		wantErr bool
	}{
		{
			name: "input, file and column",
			raw:  "ignored_paths=exception.csv:Path",
			want: &PolicyInputFromFile{Input: "ignored_paths", Column: "Path", File: "exception.csv"},
		},
		{
			name: "column defaults to input name",
			raw:  "ignored_paths=exception.csv",
			want: &PolicyInputFromFile{Input: "ignored_paths", Column: "ignored_paths", File: "exception.csv"},
		},
		{
			name: "windows drive letter without column is not split",
			raw:  `ignored_paths=C:\data\exception.csv`,
			want: &PolicyInputFromFile{Input: "ignored_paths", Column: "ignored_paths", File: `C:\data\exception.csv`},
		},
		{
			name: "windows drive letter with column",
			raw:  `ignored_paths=C:\data\exception.csv:Path`,
			want: &PolicyInputFromFile{Input: "ignored_paths", Column: "Path", File: `C:\data\exception.csv`},
		},
		{
			name: "url file without column is not split",
			raw:  "ignored_paths=https://example.com/exception.csv",
			want: &PolicyInputFromFile{Input: "ignored_paths", Column: "ignored_paths", File: "https://example.com/exception.csv"},
		},
		{
			name: "column with a space",
			raw:  "versions=exception.csv:Product Version",
			want: &PolicyInputFromFile{Input: "versions", Column: "Product Version", File: "exception.csv"},
		},
		{
			name: "surrounding whitespace trimmed",
			raw:  " ignored_paths = exception.csv : Path ",
			want: &PolicyInputFromFile{Input: "ignored_paths", Column: "Path", File: "exception.csv"},
		},
		{
			name:    "missing equals",
			raw:     "ignored_paths:Path",
			wantErr: true,
		},
		{
			name:    "missing input name",
			raw:     "=exception.csv",
			wantErr: true,
		},
		{
			name:    "missing file",
			raw:     "ignored_paths:Path=",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePolicyInputFromFile(tc.raw)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractColumnValues(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
		content  string
		column   string
		want     []string
		wantErr  bool
	}{
		{
			name:     "CSV pulls named column case-insensitively",
			filename: "exception.csv",
			content:  "Path,Verified,Publisher\nc:\\a.dll,Signed,Acme\nc:\\b.dll,Unsigned,Acme\n",
			column:   "path",
			want:     []string{"c:\\a.dll", "c:\\b.dll"},
		},
		{
			name:     "CSV drops empty cells",
			filename: "exception.csv",
			content:  "Path,Other\nc:\\a.dll,x\n,y\nc:\\b.dll,z\n",
			column:   "Path",
			want:     []string{"c:\\a.dll", "c:\\b.dll"},
		},
		{
			name:     "CSV missing column errors",
			filename: "exception.csv",
			content:  "Path\nc:\\a.dll\n",
			column:   "Nope",
			wantErr:  true,
		},
		{
			name:     "JSON array of strings",
			filename: "exception.json",
			content:  `["c:\\a.dll", "c:\\b.dll", ""]`,
			column:   "ignored_paths",
			want:     []string{"c:\\a.dll", "c:\\b.dll"},
		},
		{
			name:     "JSON with UTF-8 BOM prefix",
			filename: "exception.json",
			content:  "\xef\xbb\xbf[\"c:\\\\a.dll\"]",
			column:   "ignored_paths",
			want:     []string{"c:\\a.dll"},
		},
		{
			name:     "JSON array of objects",
			filename: "exception.json",
			content:  `[{"Path":"c:\\a.dll","Publisher":"Acme"},{"Path":"c:\\b.dll"}]`,
			column:   "Path",
			want:     []string{"c:\\a.dll", "c:\\b.dll"},
		},
		{
			name:     "JSON object mapping column to array",
			filename: "exception.json",
			content:  `{"ignored_paths":["c:\\a.dll","c:\\b.dll"]}`,
			column:   "ignored_paths",
			want:     []string{"c:\\a.dll", "c:\\b.dll"},
		},
		{
			name:     "JSON object missing key errors",
			filename: "exception.json",
			content:  `{"other":["x"]}`,
			column:   "ignored_paths",
			wantErr:  true,
		},
		{
			name:     "content sniff detects JSON without extension",
			filename: "exception.dat",
			content:  `["c:\\a.dll"]`,
			column:   "ignored_paths",
			want:     []string{"c:\\a.dll"},
		},
		{
			name:     "content sniff falls back to CSV without extension",
			filename: "exception.dat",
			content:  "Path\nc:\\a.dll\n",
			column:   "Path",
			want:     []string{"c:\\a.dll"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.filename)
			require.NoError(t, os.WriteFile(path, []byte(tc.content), 0600))

			got, err := ExtractColumnValues(path, tc.column)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
