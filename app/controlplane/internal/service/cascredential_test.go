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

package service

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSourceInternal(t *testing.T) {
	testCases := []struct {
		name      string
		requested bool
		token     *entities.APIToken
		want      bool
		wantErr   bool
	}{
		{
			name:      "not requested, no token (user auth)",
			requested: false,
			token:     nil,
			want:      false,
		},
		{
			name:      "not requested, regular API token",
			requested: false,
			token:     &entities.APIToken{},
			want:      false,
		},
		{
			name:      "not requested, system API token",
			requested: false,
			token:     &entities.APIToken{IsSystem: true},
			want:      false,
		},
		{
			name:      "requested by system API token",
			requested: true,
			token:     &entities.APIToken{IsSystem: true},
			want:      true,
		},
		{
			name:      "requested by regular API token is forbidden",
			requested: true,
			token:     &entities.APIToken{},
			wantErr:   true,
		},
		{
			name:      "requested by user auth is forbidden",
			requested: true,
			token:     nil,
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveSourceInternal(tc.requested, tc.token)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errors.IsForbidden(err))
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
