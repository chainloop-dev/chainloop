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
		want  ParsedToken
	}{
		{
			name:  "robot account",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJvcmdfaWQiOiI5M2QwMjI3NS04NTNjLTRhZDYtOWQ2MC04ZjU2MmIxMjNmZDIiLCJ3b3JrZmxvd19pZCI6IjM1ZTZkOGIwLWE0OGYtNDFmYS05YmU3LWQ1OTM5YjJkZGUyNiIsImlzcyI6ImNwLmNoYWlubG9vcCIsImF1ZCI6WyJhdHRlc3RhdGlvbnMuY2hhaW5sb29wIl0sImp0aSI6IjQ4YWVhMWNiLTk5MGUtNDM2OS1hOTFhLTczNTIzNzk1NjhiNSJ9.aYPAPK-AauJpxRGEapV2ejwxhyhmFej6S79Ni-xJ4Q0",
			want: ParsedToken{
				Type: "robot-account",
			},
		},
		{
			name:  "user account",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiYmMwYjIxOTktY2E4NS00MmFiLWE4NTctMDQyZTljMTA5ZDQzIiwiaXNzIjoiY3AuY2hhaW5sb29wIiwiYXVkIjpbInVzZXItYXV0aC5jaGFpbmxvb3AiXSwiZXhwIjoxNzE1OTM1MjUwfQ.ounYshGtagtYQsVIzNeE0ztVYRXrmjFSpdmaTF4QvyY",
			want: ParsedToken{
				ID:   "bc0b2199-ca85-42ab-a857-042e9c109d43",
				Type: "user",
			},
		},
		{
			name:  "api token",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJjcC5jaGFpbmxvb3AiLCJhdWQiOlsiYXBpLXRva2VuLWF1dGguY2hhaW5sb29wIl0sImp0aSI6IjE0NTQ5MzUwLTFjNGItNDdlYi05NDZkLWY2MjJhZWYyMDk0MyJ9.BPzuNxSwx10h22fJ3ocAOEIjsq9OOlk9p8fSoCwqSmM",
			want: ParsedToken{
				Type: "api-token",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseToken(tt.token)
			assert.NoError(t, err)

			assert.Equal(t, tt.want.ID, got.ID)
			assert.Equal(t, tt.want.Type, got.Type)
		})
	}
}
