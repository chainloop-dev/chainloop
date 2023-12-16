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
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireCurrentUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("no user", func(t *testing.T) {
		_, err := requireCurrentUser(ctx)
		assert.Error(t, err)
	})

	t.Run("with user", func(t *testing.T) {
		want := &usercontext.User{}
		ctx = usercontext.WithCurrentUser(ctx, want)
		u, err := requireCurrentUser(ctx)
		assert.NoError(t, err)
		require.Equal(t, want, u)
	})
}

func TestRequireCurrentOrg(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("no org", func(t *testing.T) {
		_, err := requireCurrentOrg(ctx)
		assert.Error(t, err)
	})

	t.Run("with org", func(t *testing.T) {
		want := &usercontext.Org{}
		ctx = usercontext.WithCurrentOrg(ctx, want)
		o, err := requireCurrentOrg(ctx)
		assert.NoError(t, err)
		require.Equal(t, want, o)
	})
}

func TestRequireAPIToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("no token", func(t *testing.T) {
		_, err := requireAPIToken(ctx)
		assert.Error(t, err)
	})

	t.Run("with token", func(t *testing.T) {
		want := &usercontext.APIToken{}
		ctx = usercontext.WithCurrentAPIToken(ctx, want)
		got, err := requireAPIToken(ctx)
		assert.NoError(t, err)
		require.Equal(t, want, got)
	})
}

func TestRequireCurrentUserOrAPIToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tesCases := []struct {
		name     string
		hasUser  bool
		hasToken bool
		wantErr  bool
	}{
		{
			name:     "no user nor token",
			hasUser:  false,
			hasToken: false,
			wantErr:  true,
		},
		{
			name:     "with user",
			hasUser:  true,
			hasToken: false,
			wantErr:  false,
		},
		{
			name:     "with token",
			hasUser:  false,
			hasToken: true,
			wantErr:  false,
		},
	}

	for _, tc := range tesCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = context.Background()
			wantUser := &usercontext.User{}
			wantToken := &usercontext.APIToken{}

			if tc.hasUser {
				ctx = usercontext.WithCurrentUser(ctx, wantUser)
			}

			if tc.hasToken {
				ctx = usercontext.WithCurrentAPIToken(ctx, wantToken)
			}

			gotUser, gotToken, err := requireCurrentUserOrAPIToken(ctx)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tc.hasUser {
				require.Equal(t, wantUser, gotUser)
			} else {
				assert.Nil(t, gotUser)
			}

			if tc.hasToken {
				require.Equal(t, wantToken, gotToken)
			} else {
				assert.Nil(t, gotToken)
			}
		})
	}

}
