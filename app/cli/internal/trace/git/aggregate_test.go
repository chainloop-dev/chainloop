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

package git

import (
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
)

func TestAddCommitStats_Status(t *testing.T) {
	testCases := []struct {
		name     string
		statuses []string
		want     string
	}{
		{name: "one commit", statuses: []string{StatusModified}, want: StatusModified},
		{name: "a file that the commits create stays created", statuses: []string{StatusCreated, StatusModified, StatusModified}, want: StatusCreated},
		{name: "a created file that a later commit deletes is deleted", statuses: []string{StatusCreated, StatusDeleted}, want: StatusDeleted},
		{name: "the last status wins otherwise", statuses: []string{StatusModified, StatusDeleted}, want: StatusDeleted},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := &aicodingsession.CodeChanges{}
			byPath := make(map[string]*fileAgg)
			for _, s := range tc.statuses {
				addCommitStats(out, byPath, &aicodingsession.CodeChanges{Files: []aicodingsession.FileChange{{Path: "a.txt", Status: s}}})
			}
			finalizeAggregatedFiles(out, byPath)

			assert.Equal(t, tc.want, out.Files[0].Status)
		})
	}
}
