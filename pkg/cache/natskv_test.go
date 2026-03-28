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
	"strings"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startEmbeddedNATS(t *testing.T) *nats.Conn {
	t.Helper()
	opts := &natsserver.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}
	ns, err := natsserver.NewServer(opts)
	require.NoError(t, err)
	ns.Start()
	t.Cleanup(ns.Shutdown)

	require.True(t, ns.ReadyForConnections(5*time.Second))
	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	return nc
}

// sanitizeBucketName makes test names safe for NATS bucket names (alphanumeric, dash, underscore only).
func sanitizeBucketName(name string) string {
	r := strings.NewReplacer("/", "-", " ", "-", "=", "-")
	return r.Replace(name)
}

type testStruct struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestNATSKV_GetSetDelete(t *testing.T) {
	nc := startEmbeddedNATS(t)

	tests := []struct {
		name string
		run  func(t *testing.T, c Cache[testStruct])
	}{
		{
			name: "get from empty cache returns miss",
			run: func(t *testing.T, c Cache[testStruct]) {
				_, ok, err := c.Get(context.Background(), "missing")
				require.NoError(t, err)
				assert.False(t, ok)
			},
		},
		{
			name: "set then get returns value",
			run: func(t *testing.T, c Cache[testStruct]) {
				ctx := context.Background()
				v := testStruct{Name: "test", Value: 42}
				require.NoError(t, c.Set(ctx, "key1", v))
				got, ok, err := c.Get(ctx, "key1")
				require.NoError(t, err)
				assert.True(t, ok)
				assert.Equal(t, v, got)
			},
		},
		{
			name: "delete removes entry",
			run: func(t *testing.T, c Cache[testStruct]) {
				ctx := context.Background()
				require.NoError(t, c.Set(ctx, "key1", testStruct{Name: "x", Value: 1}))
				require.NoError(t, c.Delete(ctx, "key1"))
				_, ok, err := c.Get(ctx, "key1")
				require.NoError(t, err)
				assert.False(t, ok)
			},
		},
		{
			name: "purge removes all entries",
			run: func(t *testing.T, c Cache[testStruct]) {
				ctx := context.Background()
				require.NoError(t, c.Set(ctx, "k1", testStruct{Name: "a", Value: 1}))
				require.NoError(t, c.Set(ctx, "k2", testStruct{Name: "b", Value: 2}))
				require.NoError(t, c.Purge(ctx))
				_, ok1, _ := c.Get(ctx, "k1")
				_, ok2, _ := c.Get(ctx, "k2")
				assert.False(t, ok1)
				assert.False(t, ok2)
			},
		},
		{
			name: "key sanitization replaces colons with dots",
			run: func(t *testing.T, c Cache[testStruct]) {
				ctx := context.Background()
				v := testStruct{Name: "sanitized", Value: 99}
				require.NoError(t, c.Set(ctx, "token:org:name", v))
				got, ok, err := c.Get(ctx, "token:org:name")
				require.NoError(t, err)
				assert.True(t, ok)
				assert.Equal(t, v, got)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New[testStruct](
				WithTTL(5*time.Second),
				WithNATS(nc, sanitizeBucketName("test-"+t.Name())),
			)
			require.NoError(t, err)
			tt.run(t, c)
		})
	}
}

func TestNATSKV_TTLExpiration(t *testing.T) {
	nc := startEmbeddedNATS(t)
	c, err := New[string](
		WithTTL(200*time.Millisecond),
		WithNATS(nc, "test-ttl"),
	)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "ephemeral", "gone-soon"))
	_, ok, _ := c.Get(ctx, "ephemeral")
	require.True(t, ok)

	time.Sleep(400 * time.Millisecond)
	_, ok, err = c.Get(ctx, "ephemeral")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestNATSKV_CorruptedDataReturnsMiss(t *testing.T) {
	nc := startEmbeddedNATS(t)
	c, err := New[testStruct](
		WithTTL(5*time.Second),
		WithNATS(nc, "test-corrupt"),
	)
	require.NoError(t, err)

	// Write garbage directly to the KV bucket
	nkv := c.(*natsKVCache[testStruct])
	nkv.mu.RLock()
	kv := nkv.kv
	nkv.mu.RUnlock()
	_, err = kv.PutString(context.Background(), "corrupt-key", "not-valid-json{{{")
	require.NoError(t, err)

	// Get should return a miss (and auto-delete the corrupted entry)
	_, ok, err := c.Get(context.Background(), "corrupt-key")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestNATSKV_NilKVGracefulDegradation(t *testing.T) {
	nc := startEmbeddedNATS(t)
	c, err := New[string](
		WithTTL(time.Second),
		WithNATS(nc, "test-degradation"),
	)
	require.NoError(t, err)

	// Simulate nil KV handle
	nkv := c.(*natsKVCache[string])
	nkv.mu.Lock()
	nkv.kv = nil
	nkv.mu.Unlock()

	ctx := context.Background()
	_, ok, err := nkv.Get(ctx, "key")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, nkv.Set(ctx, "key", "val"))
	require.NoError(t, nkv.Delete(ctx, "key"))
	require.NoError(t, nkv.Purge(ctx))
}

func TestNew_WithNATSReturnsNATSBackend(t *testing.T) {
	nc := startEmbeddedNATS(t)
	c, err := New[string](
		WithTTL(time.Second),
		WithNATS(nc, "test-backend-select"),
	)
	require.NoError(t, err)
	_, ok := c.(*natsKVCache[string])
	assert.True(t, ok, "expected natsKVCache when NATS connection provided")
}
