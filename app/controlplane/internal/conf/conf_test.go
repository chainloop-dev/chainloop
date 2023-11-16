//
// Copyright 2023 The Chainloop Authors.
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

package conf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateOrgs(t *testing.T) {
	testCases := []struct {
		name       string
		index      *ReferrerSharedIndex
		wantErrMsg string
	}{
		{
			name:  "nil configuration",
			index: nil,
		},
		{
			name: "enabled but without orgs",
			index: &ReferrerSharedIndex{
				Enabled: true,
			},
			wantErrMsg: "index is enabled, but no orgs are allowed",
		},
		{
			name: "enabled with invalid orgs",
			index: &ReferrerSharedIndex{
				Enabled:     true,
				AllowedOrgs: []string{"invalid"},
			},
			wantErrMsg: "invalid org id: invalid",
		},
		{
			name: "with invalid orgs but disabled",
			index: &ReferrerSharedIndex{
				Enabled:     false,
				AllowedOrgs: []string{"invalid"},
			},
		},
		{
			name: "enabled with valid orgs",
			index: &ReferrerSharedIndex{
				Enabled:     true,
				AllowedOrgs: []string{"00000000-0000-0000-0000-000000000000"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.index.ValidateOrgs()
			if tc.wantErrMsg != "" {
				assert.EqualError(t, err, tc.wantErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
