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

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  *parsedToken
	}{
		{
			name:  "user account",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiYmMwYjIxOTktY2E4NS00MmFiLWE4NTctMDQyZTljMTA5ZDQzIiwiaXNzIjoiY3AuY2hhaW5sb29wIiwiYXVkIjpbInVzZXItYXV0aC5jaGFpbmxvb3AiXSwiZXhwIjoxNzE1OTM1MjUwfQ.ounYshGtagtYQsVIzNeE0ztVYRXrmjFSpdmaTF4QvyY",
			want: &parsedToken{
				id:        "bc0b2199-ca85-42ab-a857-042e9c109d43",
				tokenType: "user",
			},
		},
		{
			name:  "api token",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJvcmdfaWQiOiJkZGRiODIwMS1lYWI2LTRlNjEtOTIwMS1mMTJiNDdjMDE4OTIiLCJpc3MiOiJjcC5jaGFpbmxvb3AiLCJhdWQiOlsiYXBpLXRva2VuLWF1dGguY2hhaW5sb29wIl0sImp0aSI6IjRiMGYwZGQ0LTQ1MzgtNDI2OS05MmE5LWFiNWIwZmNlMDI1OCJ9.yMgsoe4CcqYoNp0xtrvvSGj1Y74HeqxoxS5sw8pdnQ8",
			want: &parsedToken{
				id:        "4b0f0dd4-4538-4269-92a9-ab5b0fce0258",
				tokenType: "api-token",
				orgID:     "dddb8201-eab6-4e61-9201-f12b47c01892",
			},
		},
		{
			name:  "old api token (without orgID)",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJjcC5jaGFpbmxvb3AiLCJhdWQiOlsiYXBpLXRva2VuLWF1dGguY2hhaW5sb29wIl0sImp0aSI6ImQ0ZTBlZTVlLTk3MTMtNDFkMi05ZmVhLTBiZGIxNDAzMzA4MSJ9.IOd3JIHPwfo9ihU20kvRwLIQJcQtTvp-ajlGqlCD4Es",
			want: &parsedToken{
				id:        "d4e0ee5e-9713-41d2-9fea-0bdb14033081",
				tokenType: "api-token",
			},
		},
		{
			name:  "totally random token",
			token: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE3MTYxOTE2ODQsImV4cCI6MTc0NzcyNzY4NCwiYXVkIjoid3d3LmV4YW1wbGUuY29tIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIkdpdmVuTmFtZSI6IkpvaG5ueSIsIlN1cm5hbWUiOiJSb2NrZXQiLCJFbWFpbCI6Impyb2NrZXRAZXhhbXBsZS5jb20iLCJSb2xlIjpbIk1hbmFnZXIiLCJQcm9qZWN0IEFkbWluaXN0cmF0b3IiXX0.5UnBivwkQCG4qWgi-gWkJ-Dsd7-A9G_EVEvswODc7Kk",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseToken(tt.token)
			assert.NoError(t, err)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			assert.Equal(t, tt.want.id, got.id)
			assert.Equal(t, tt.want.tokenType, got.tokenType)
			assert.Equal(t, tt.want.orgID, got.orgID)
		})
	}
}
