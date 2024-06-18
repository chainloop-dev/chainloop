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

package service

import (
	"testing"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/stretchr/testify/assert"
)

func TestGetAuthURLs(t *testing.T) {
	internalServer := &conf.Server_HTTP{Addr: "1.2.3.4"}
	testCases := []struct {
		name             string
		config           *conf.Server_HTTP
		loginURLOverride string
		want             *AuthURLs
		wantErr          bool
	}{
		{
			name:   "neither external url nor externalAddr set",
			config: internalServer,
			want:   &AuthURLs{callback: "http://1.2.3.4/auth/callback", Login: "http://1.2.3.4/auth/login"},
		},
		{
			name:   "correct URL, http",
			config: &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "http://foo.com"},
			want:   &AuthURLs{callback: "http://foo.com/auth/callback", Login: "http://foo.com/auth/login"},
		},
		{
			name:   "correct URL, https",
			config: &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "https://foo.com"},
			want:   &AuthURLs{callback: "https://foo.com/auth/callback", Login: "https://foo.com/auth/login"},
		},
		{
			name:   "with path",
			config: &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "https://foo.com/path"},
			want:   &AuthURLs{callback: "https://foo.com/path/auth/callback", Login: "https://foo.com/path/auth/login"},
		},
		{
			name:   "with port",
			config: &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "https://foo.com:1234"},
			want:   &AuthURLs{callback: "https://foo.com:1234/auth/callback", Login: "https://foo.com:1234/auth/login"},
		},
		{
			name:    "invalid, missing scheme",
			config:  &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "localhost.com"},
			wantErr: true,
		},
		{
			name:             "external with override",
			config:           &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "https://foo.com"},
			loginURLOverride: "https://foo.override.com/auth/login",
			want:             &AuthURLs{callback: "https://foo.com/auth/callback", Login: "https://foo.override.com/auth/login"},
		},
		{
			name:             "internal with override",
			config:           internalServer,
			loginURLOverride: "https://foo.override.com/auth/login",
			want:             &AuthURLs{callback: "http://1.2.3.4/auth/callback", Login: "https://foo.override.com/auth/login"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getAuthURLs(tc.config, tc.loginURLOverride)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
