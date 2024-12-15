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

package junit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIngest(t *testing.T) {
	cases := []struct {
		name      string
		filename  string
		nSuites   int
		expectErr bool
	}{
		{
			name:     "single junit report",
			filename: "../testdata/junit.xml",
			nSuites:  2,
		},
		{
			name:     "zipped reports",
			filename: "../testdata/tests.zip",
			nSuites:  13,
		},
		{
			name:     "gzipped reports",
			filename: "../testdata/tests.tar.gz",
			nSuites:  13,
		},
		{
			name:      "invalid xml",
			filename:  "../testdata/junit-invalid.xml",
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			suites, err := Ingest(tc.filename)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.nSuites, len(suites))
		})
	}
}
