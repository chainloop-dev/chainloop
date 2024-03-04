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

package usercontext

import (
	"context"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/stretchr/testify/assert"
)

func TestCheckUserInAllowList(t *testing.T) {
	const email = "sarah@cyberdyne.io"
	allowList := []string{
		"foo@foo.com",
		"sarah@cyberdyne.io",
		// it can also contain domains
		"@cyberdyne.io",
		"@dyson-industries.io",
	}

	testCases := []struct {
		name      string
		allowList []string
		email     string
		wantErr   bool
	}{
		{
			name:      "empty allow list",
			email:     email,
			allowList: []string{},
		},
		{
			name:      "user not in allow list",
			email:     email,
			allowList: []string{"nothere@cyberdyne.io"},
			wantErr:   true,
		},
		{
			name:      "context missing, no user loaded",
			allowList: allowList,
			wantErr:   true,
		},
		{
			name:      "user in allow list",
			email:     email,
			allowList: allowList,
		},
		{
			name:      "user in one of the valid domains",
			email:     "miguel@dyson-industries.io",
			allowList: allowList,
		},
		{
			name:      "user in one of the valid domains",
			email:     "john@dyson-industries.io",
			allowList: allowList,
		},
		{
			name:      "and can use modifiers",
			email:     "john+chainloop@dyson-industries.io",
			allowList: allowList,
		},
		{
			name:      "it needs to be an email",
			email:     "dyson-industries.io",
			allowList: allowList,
			wantErr:   true,
		},
		{
			name:      "domain position is important",
			email:     "dyson-industries.io@john",
			allowList: allowList,
			wantErr:   true,
		},
		{
			name:      "and can't be typosquated",
			email:     "john@dyson-industriesss.io",
			allowList: allowList,
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := CheckUserInAllowList(tc.allowList)
			ctx := context.Background()
			if tc.email != "" {
				u := &User{Email: tc.email, ID: "124"}
				ctx = WithCurrentUser(ctx, u)
			}

			_, err := m(emptyHandler)(ctx, nil)

			if tc.wantErr {
				assert.True(t, v1.IsAllowListErrorNotInList(err))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
