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

package policies

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRef(t *testing.T) {
	testCases := []struct {
		name       string
		policyURL  string
		policyName string
		digest     string
		orgName    string
		want       *PolicyReference
	}{
		{
			name:       "base",
			policyURL:  "https://p1host.com/foo",
			policyName: "my-policy",
			digest:     "my-digest",
			want:       &PolicyReference{URL: "chainloop://p1host.com/my-policy", Digest: "my-digest"},
		},
		{
			name:       "with org",
			policyURL:  "https://p1host.com/foo",
			policyName: "my-policy",
			digest:     "my-digest",
			orgName:    "my-org",
			want:       &PolicyReference{URL: "chainloop://p1host.com/my-policy?org=my-org", Digest: "my-digest"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policyURL, err := url.Parse(tc.policyURL)
			require.NoError(t, err)
			got := createRef(policyURL, tc.policyName, tc.digest, tc.orgName)

			assert.Equal(t, tc.want, got)
		})
	}
}
