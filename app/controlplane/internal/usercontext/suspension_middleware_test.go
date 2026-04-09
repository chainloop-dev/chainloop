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

package usercontext

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var passHandler = func(_ context.Context, _ interface{}) (interface{}, error) { return "ok", nil }

func TestWithSuspensionMiddleware(t *testing.T) {
	suspendedOrg := &entities.Org{ID: "org-1", Name: "test", Suspended: true}
	activeOrg := &entities.Org{ID: "org-1", Name: "test", Suspended: false}

	tests := []struct {
		name    string
		org     *entities.Org
		wantErr bool
	}{
		{
			name:    "no org context passes through",
			org:     nil,
			wantErr: false,
		},
		{
			name:    "active org passes through",
			org:     activeOrg,
			wantErr: false,
		},
		{
			name:    "suspended org is blocked",
			org:     suspendedOrg,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.org != nil {
				ctx = entities.WithCurrentOrg(ctx, tt.org)
			}

			m := WithSuspensionMiddleware()
			result, err := m(passHandler)(ctx, nil)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "suspended")
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "ok", result)
			}
		})
	}
}
