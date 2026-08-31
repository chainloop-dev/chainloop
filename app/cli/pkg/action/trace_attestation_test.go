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
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckTokenOrganization(t *testing.T) {
	apiToken := func(t *testing.T, org string) string {
		t.Helper()
		claims := jwt.MapClaims{"aud": "api-token-auth.chainloop", "jti": "some-id"}
		if org != "" {
			claims["org_name"] = org
		}
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
		require.NoError(t, err)

		return s
	}

	t.Run("API token from another org is rejected", func(t *testing.T) {
		err := checkTokenOrganization(apiToken(t, "foo"), "bar")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "foo")
		assert.Contains(t, err.Error(), "bar")
	})

	t.Run("API token from the pinned org passes", func(t *testing.T) {
		assert.NoError(t, checkTokenOrganization(apiToken(t, "bar"), "bar"))
		assert.NoError(t, checkTokenOrganization(apiToken(t, "BAR"), "bar"))
	})

	t.Run("tokens without an org claim pass", func(t *testing.T) {
		// User and federated tokens: the org header is honored server-side.
		assert.NoError(t, checkTokenOrganization(apiToken(t, ""), "bar"))
		assert.NoError(t, checkTokenOrganization("", "bar"))
		assert.NoError(t, checkTokenOrganization("not-a-jwt", "bar"))
	})
}
