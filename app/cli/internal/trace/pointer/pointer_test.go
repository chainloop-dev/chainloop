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

package pointer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDigest(t *testing.T) {
	testCases := []struct {
		name    string
		content string
		want    string
	}{
		{name: "empty", content: "", want: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{name: "text", content: "a", want: "sha256:ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Digest([]byte(tc.content)))

			fromReader, err := DigestReader(strings.NewReader(tc.content))
			require.NoError(t, err)
			assert.Equal(t, tc.want, fromReader, "both forms give the same digest")
		})
	}
}

func TestNew(t *testing.T) {
	assert.JSONEq(t, `{"type":"chainloop.replaced","digest":"sha256:abc"}`, string(New("sha256:abc")))
}

func TestReport(t *testing.T) {
	report := Report{}
	report.Finder(FinderFileRead).Skip(ReasonFileGone, 2)
	report.Finder(FinderFileRead).Skip(ReasonFileGone, 1)
	report.Finder(FinderFileRead).Skip(ReasonPartialRead, 0)
	report.Finder(FinderFileWrite).Replaced++

	assert.Equal(t, Report{
		FinderFileRead:  {Skipped: map[string]int{ReasonFileGone: 3}},
		FinderFileWrite: {Replaced: 1},
	}, report)
}
