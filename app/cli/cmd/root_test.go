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

package cmd_test

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/cmd"
	"github.com/stretchr/testify/assert"
)

func TestParseToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  cmd.ParsedToken
	}{
		{
			name:  "robot account",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJvcmdfaWQiOiI5M2QwMjI3NS04NTNjLTRhZDYtOWQ2MC04ZjU2MmIxMjNmZDIiLCJ3b3JrZmxvd19pZCI6IjM1ZTZkOGIwLWE0OGYtNDFmYS05YmU3LWQ1OTM5YjJkZGUyNiIsImlzcyI6ImNwLmNoYWlubG9vcCIsImF1ZCI6WyJhdHRlc3RhdGlvbnMuY2hhaW5sb29wIl0sImp0aSI6IjQ4YWVhMWNiLTk5MGUtNDM2OS1hOTFhLTczNTIzNzk1NjhiNSJ9.aYPAPK-AauJpxRGEapV2ejwxhyhmFej6S79Ni-xJ4Q0",
			want: cmd.ParsedToken{
				Type: "robot-account",
			},
		},
		{
			name:  "user account",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiYmMwYjIxOTktY2E4NS00MmFiLWE4NTctMDQyZTljMTA5ZDQzIiwiaXNzIjoiY3AuY2hhaW5sb29wIiwiYXVkIjpbInVzZXItYXV0aC5jaGFpbmxvb3AiXSwiZXhwIjoxNzE1OTM1MjUwfQ.ounYshGtagtYQsVIzNeE0ztVYRXrmjFSpdmaTF4QvyY",
			want: cmd.ParsedToken{
				ID:   "bc0b2199-ca85-42ab-a857-042e9c109d43",
				Type: "user",
			},
		},
		{
			name:  "api token",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJjcC5jaGFpbmxvb3AiLCJhdWQiOlsiYXBpLXRva2VuLWF1dGguY2hhaW5sb29wIl0sImp0aSI6IjE0NTQ5MzUwLTFjNGItNDdlYi05NDZkLWY2MjJhZWYyMDk0MyJ9.BPzuNxSwx10h22fJ3ocAOEIjsq9OOlk9p8fSoCwqSmM",
			want: cmd.ParsedToken{
				Type: "api-token",
			},
		},
		{
			name:  "random token",
			token: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE3MTU4NTQ0MDksImV4cCI6MTc0NzM5MDQwOSwiYXVkIjoid3d3LmV4YW1wbGUuY29tIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIkdpdmVuTmFtZSI6IkpvaG5ueSIsIlN1cm5hbWUiOiJSb2NrZXQiLCJFbWFpbCI6Impyb2NrZXRAZXhhbXBsZS5jb20iLCJSb2xlIjpbIk1hbmFnZXIiLCJQcm9qZWN0IEFkbWluaXN0cmF0b3IiXX0.dBfZb24Q8Q3JIYQddAaGEkYMvxnqctGgRCeY6Z1Qx8A",
			want:  cmd.ParsedToken{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cmd.ParseToken(tt.token)
			assert.NoError(t, err)

			assert.Equal(t, tt.want.ID, got.ID)
			assert.Equal(t, tt.want.Type, got.Type)
		})
	}
}
