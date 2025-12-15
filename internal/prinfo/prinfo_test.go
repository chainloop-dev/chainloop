//
// Copyright 2025 The Chainloop Authors.
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

package prinfo

import (
	"encoding/json"
	"testing"

	"github.com/chainloop-dev/chainloop/internal/schemavalidators"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePRInfo(t *testing.T) {
	testCases := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name: "valid GitHub PR",
			data: `{
				"platform": "github",
				"type": "pull_request",
				"number": "123",
				"url": "https://github.com/owner/repo/pull/123",
				"title": "Add new feature",
				"description": "This PR adds a new feature",
				"source_branch": "feature-branch",
				"target_branch": "main",
				"author": "username"
			}`,
			wantErr: false,
		},
		{
			name: "valid GitLab MR minimal",
			data: `{
				"platform": "gitlab",
				"type": "merge_request",
				"number": "456",
				"url": "https://gitlab.com/owner/repo/-/merge_requests/456"
			}`,
			wantErr: false,
		},
		{
			name: "missing required field: platform",
			data: `{
				"type": "pull_request",
				"number": "123",
				"url": "https://github.com/owner/repo/pull/123"
			}`,
			wantErr: true,
		},
		{
			name: "missing required field: url",
			data: `{
				"platform": "github",
				"type": "pull_request",
				"number": "123"
			}`,
			wantErr: true,
		},
		{
			name: "invalid platform value",
			data: `{
				"platform": "bitbucket",
				"type": "pull_request",
				"number": "123",
				"url": "https://bitbucket.org/owner/repo/pull/123"
			}`,
			wantErr: true,
		},
		{
			name: "invalid type value",
			data: `{
				"platform": "github",
				"type": "issue",
				"number": "123",
				"url": "https://github.com/owner/repo/pull/123"
			}`,
			wantErr: true,
		},
		{
			name: "additional properties not allowed",
			data: `{
				"platform": "github",
				"type": "pull_request",
				"number": "123",
				"url": "https://github.com/owner/repo/pull/123",
				"extra_field": "not allowed"
			}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			data:    `{invalid json}`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var data interface{}
			err := json.Unmarshal([]byte(tc.data), &data)
			if err != nil {
				// For invalid JSON test case
				if tc.wantErr {
					return
				}
				require.NoError(t, err)
			}

			err = schemavalidators.ValidatePRInfo(data, schemavalidators.PRInfoVersion1_0)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
