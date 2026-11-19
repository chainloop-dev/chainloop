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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadOutputPath(t *testing.T) {
	downloadDir := filepath.Join(string(filepath.Separator), "home", "user", "downloads")

	testCases := []struct {
		name     string
		filename string
		want     string
		wantErr  bool
	}{
		{name: "plain filename", filename: "sbom.json", want: "sbom.json"},
		{name: "filename with spaces", filename: "my report.sarif", want: "my report.sarif"},
		{name: "dotfile", filename: ".env.sample", want: ".env.sample"},
		{name: "relative traversal", filename: "../../../escaped.txt", want: "escaped.txt"},
		{name: "nested path", filename: "dir/sub/file.txt", want: "file.txt"},
		{name: "absolute path", filename: "/etc/passwd", want: "passwd"},
		{name: "windows separators", filename: `..\..\escaped.txt`, want: "escaped.txt"},
		{name: "mixed separators", filename: `a/..\b/../c.txt`, want: "c.txt"},
		{name: "trailing separator", filename: "dir/", want: "dir"},
		{name: "empty", filename: "", wantErr: true},
		{name: "dot", filename: ".", wantErr: true},
		{name: "dot dot", filename: "..", wantErr: true},
		{name: "traversal ending in dot dot", filename: "../..", wantErr: true},
		{name: "root only", filename: "/", wantErr: true},
		{name: "backslash only", filename: `\`, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := downloadOutputPath(downloadDir, tc.filename)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, filepath.Join(downloadDir, tc.want), got)
			// The result never leaves the download directory
			assert.Equal(t, downloadDir, filepath.Dir(got))
		})
	}
}
