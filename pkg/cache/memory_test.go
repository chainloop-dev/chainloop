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

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCache_GetSetDelete(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, c Cache[string])
	}{
		{
			name: "get from empty cache returns miss",
			run: func(t *testing.T, c Cache[string]) {
				_, ok, err := c.Get(context.Background(), "missing")
				require.NoError(t, err)
				assert.False(t, ok)
			},
		},
		{
			name: "set then get returns value",
			run: func(t *testing.T, c Cache[string]) {
				ctx := context.Background()
				require.NoError(t, c.Set(ctx, "key1", "value1"))
				val, ok, err := c.Get(ctx, "key1")
				require.NoError(t, err)
				assert.True(t, ok)
				assert.Equal(t, "value1", val)
			},
		},
		{
			name: "delete removes entry",
			run: func(t *testing.T, c Cache[string]) {
				ctx := context.Background()
				require.NoError(t, c.Set(ctx, "key1", "value1"))
				require.NoError(t, c.Delete(ctx, "key1"))
				_, ok, err := c.Get(ctx, "key1")
				require.NoError(t, err)
				assert.False(t, ok)
			},
		},
		{
			name: "purge removes all entries",
			run: func(t *testing.T, c Cache[string]) {
				ctx := context.Background()
				require.NoError(t, c.Set(ctx, "k1", "v1"))
				require.NoError(t, c.Set(ctx, "k2", "v2"))
				require.NoError(t, c.Purge(ctx))
				_, ok1, _ := c.Get(ctx, "k1")
				_, ok2, _ := c.Get(ctx, "k2")
				assert.False(t, ok1)
				assert.False(t, ok2)
			},
		},
		{
			name: "TTL expiration",
			run: func(t *testing.T, c Cache[string]) {
				ctx := context.Background()
				require.NoError(t, c.Set(ctx, "ephemeral", "gone-soon"))
				time.Sleep(100 * time.Millisecond)
				_, ok, err := c.Get(ctx, "ephemeral")
				require.NoError(t, err)
				assert.False(t, ok)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New[string](WithTTL(50 * time.Millisecond))
			require.NoError(t, err)
			tt.run(t, c)
		})
	}
}

func TestNew_DefaultsToMemory(t *testing.T) {
	c, err := New[string](WithTTL(time.Second))
	require.NoError(t, err)
	_, ok := c.(*memoryCache[string])
	assert.True(t, ok, "expected memoryCache when no NATS connection provided")
}

func TestNew_RequiresTTL(t *testing.T) {
	_, err := New[string]()
	require.Error(t, err)
}
