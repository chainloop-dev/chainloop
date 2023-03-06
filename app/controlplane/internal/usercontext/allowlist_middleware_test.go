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

package usercontext

import (
	"context"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/stretchr/testify/assert"
)

func TestCheckUserInAllowList(t *testing.T) {
	u := &User{Email: "sarah@cyberdyne.io", ID: "124"}
	testCases := []struct {
		name      string
		allowList []string
		user      *User
		wantErr   bool
	}{
		{
			name:      "empty allow list",
			user:      u,
			allowList: []string{},
		},
		{
			name:      "user not in allow list",
			user:      u,
			allowList: []string{"foo@foo.com"},
			wantErr:   true,
		},
		{
			name:      "context missing, no user loaded",
			allowList: []string{"foo@foo.com"},
			wantErr:   true,
		},
		{
			name:      "user in allow list",
			user:      u,
			allowList: []string{"sarah@cyberdyne.io"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := CheckUserInAllowList(tc.allowList)
			ctx := context.Background()
			if tc.user != nil {
				ctx = withCurrentUser(ctx, tc.user)
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
