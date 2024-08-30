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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRef(t *testing.T) {
	testCases := []struct {
		providerURL string
		policyName  string
		digest      string
		want        string
		wantErr     bool
	}{
		{
			providerURL: "https://p1host.com/foo",
			policyName:  "my-policy",
			digest:      "my-digest",
			want:        "chainloop://p1host.com/my-policy@my-digest",
		},
		{
			providerURL: "https://p1host.com/foo",
			policyName:  "my-policy",
			want:        "chainloop://p1host.com/my-policy",
		},
		{
			providerURL: "p1host.com/foo",
			policyName:  "my-policy",
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.providerURL, func(t *testing.T) {
			provider := &PolicyProvider{host: tc.providerURL}

			got, err := provider.resolveRef(tc.policyName, tc.digest)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
