//
// Copyright 2025 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License a
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package biz

import (
	"testing"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/stretchr/testify/assert"
)

func TestUserEmailInAllowlist(t *testing.T) {
	const email = "sarah@cyberdyne.io"

	defaultRules := []string{
		"foo@foo.com",
		"sarah@cyberdyne.io",
		// it can also contain domains
		"@cyberdyne.io",
		"@dyson-industries.io",
	}

	testCases := []struct {
		name    string
		rules   []string
		email   string
		want    bool
		wantErr bool
	}{
		{
			name:  "empty allow list",
			email: email,
			rules: make([]string, 0),
			want:  false,
		},
		{
			name:  "user not in allow list",
			email: email,
			rules: []string{"nothere@cyberdyne.io"},
			want:  false,
		},
		{
			name:  "user in allow list",
			email: email,
			want:  true,
		},
		{
			name:  "user in one of the valid domains",
			email: "miguel@dyson-industries.io",
			want:  true,
		},
		{
			name:  "user in one of the valid domains",
			email: "john@dyson-industries.io",
			want:  true,
		},
		{
			name:  "and can use modifiers",
			email: "john+chainloop@dyson-industries.io",
			want:  true,
		},
		{
			name:    "it needs to be an email",
			email:   "dyson-industries.io",
			wantErr: true,
		},
		{
			name:  "domain position is important",
			email: "dyson-industries.io@john",
			want:  false,
		},
		{
			name:  "and can't be typosquated",
			email: "john@dyson-industriesss.io",
			want:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowList := &conf.Auth_AllowList{
				Rules: defaultRules,
			}

			if tc.rules != nil {
				allowList.Rules = tc.rules
			}

			m, err := userEmailInAllowlist(allowList, tc.email)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, m)
		})
	}
}
