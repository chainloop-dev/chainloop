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
	"github.com/nats-io/nats.go/jetstream"
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
				_, ok1, err := c.Get(ctx, "k1")
				require.NoError(t, err)
				_, ok2, err := c.Get(ctx, "k2")
				require.NoError(t, err)
				assert.False(t, ok1)
				assert.False(t, ok2)
			},
		},
		{
			name: "key sanitization encodes special characters",
			run: func(t *testing.T, c Cache[testStruct]) {
				ctx := context.Background()
				v := testStruct{Name: "sanitized", Value: 99}
				require.NoError(t, c.Set(ctx, "token:org:name", v))
				got, ok, err := c.Get(ctx, "token:org:name")
				require.NoError(t, err)
				assert.True(t, ok)
				assert.Equal(t, v, got)

				// Verify distinct keys with similar characters don't collide
				v2 := testStruct{Name: "different", Value: 100}
				require.NoError(t, c.Set(ctx, "token.org.name", v2))
				got2, ok2, err := c.Get(ctx, "token.org.name")
				require.NoError(t, err)
				assert.True(t, ok2)
				assert.Equal(t, v2, got2)

				// Original key should still have its value
				got3, ok3, err := c.Get(ctx, "token:org:name")
				require.NoError(t, err)
				assert.True(t, ok3)
				assert.Equal(t, v, got3)
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

func TestNATSKV_MaxBytesEvictsOldEntries(t *testing.T) {
	nc := startEmbeddedNATS(t)

	// With MaxBytes set, the backing stream is updated to DiscardOld so that
	// the oldest entries are evicted when the bucket is full.
	const maxBytes int64 = 10 * 1024
	c, err := New[[]byte](
		WithTTL(time.Minute),
		WithNATS(nc, "test-maxbytes"),
		WithMaxBytes(maxBytes),
	)
	require.NoError(t, err)

	ctx := context.Background()
	payload := make([]byte, 1024)

	// Write 20 entries, well beyond the 10 KiB limit
	for i := range 20 {
		key := "key-" + strings.Repeat("x", i)
		require.NoError(t, c.Set(ctx, key, payload), "Set should not fail even when bucket is full")
	}

	// The latest entry should still be retrievable
	lastKey := "key-" + strings.Repeat("x", 19)
	_, ok, err := c.Get(ctx, lastKey)
	require.NoError(t, err)
	assert.True(t, ok, "most recent entry should still be in the cache")

	// The earliest entries should have been evicted
	_, ok, err = c.Get(ctx, "key-")
	require.NoError(t, err)
	assert.False(t, ok, "oldest entry should have been evicted")
}

func TestNATSKV_EnsureDiscardOldSkipsWhenAlreadySet(t *testing.T) {
	// Exercise both branches of ensureDiscardOld directly: when the stream's
	// Discard policy already matches, UpdateStream must not be called.
	nc := startEmbeddedNATS(t)
	bucket := "test-idempotent"

	// First New creates the bucket and sets DiscardOld.
	_, err := New[[]byte](
		WithTTL(time.Minute),
		WithNATS(nc, bucket),
		WithMaxBytes(10*1024),
	)
	require.NoError(t, err)

	// Flip the backing stream back to DiscardNew so we can observe the update branch.
	js, err := jetstream.New(nc)
	require.NoError(t, err)
	streamName := "KV_" + bucket
	stream, err := js.Stream(context.Background(), streamName)
	require.NoError(t, err)
	cfg := stream.CachedInfo().Config
	cfg.Discard = jetstream.DiscardNew
	_, err = js.UpdateStream(context.Background(), cfg)
	require.NoError(t, err)

	c := &natsKVCache[[]byte]{
		logger: nopLogger{},
		conn:   nc,
		bucket: bucket,
		cfg:    &config{logger: nopLogger{}, bucketName: bucket, maxBytes: 10 * 1024},
	}

	// Update branch: must flip Discard back to DiscardOld.
	require.NoError(t, c.ensureDiscardOld(js))
	stream, err = js.Stream(context.Background(), streamName)
	require.NoError(t, err)
	require.Equal(t, jetstream.DiscardOld, stream.CachedInfo().Config.Discard)

	// Skip branch: with DiscardOld already set, ensureDiscardOld must not
	// issue an UpdateStream call. Measure outbound NATS request count across
	// a call — one Stream() lookup, zero UpdateStream() calls => 1 request.
	before := nc.Stats().OutMsgs
	require.NoError(t, c.ensureDiscardOld(js))
	delta := nc.Stats().OutMsgs - before
	assert.LessOrEqual(t, delta, uint64(1), "skip path must not issue an UpdateStream request")
}

func TestNATSKV_InitBucketRetriesOnTransientError(t *testing.T) {
	// Verify the retry wrapper gives up cleanly (returns an error, no panic)
	// when the NATS connection is unusable for the full budget. A closed
	// connection is a deterministic way to make every attempt fail.
	nc := startEmbeddedNATS(t)
	nc.Close()

	c := &natsKVCache[[]byte]{
		logger: nopLogger{},
		conn:   nc,
		bucket: "test-retry-exhausted",
		cfg: &config{
			logger:     nopLogger{},
			bucketName: "test-retry-exhausted",
			ttl:        time.Minute,
			maxBytes:   10 * 1024,
			replicas:   1,
		},
	}

	start := time.Now()
	err := c.initBucketWithRetry(200*time.Millisecond, 50*time.Millisecond)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.GreaterOrEqual(t, elapsed, 200*time.Millisecond, "should have retried for at least the budget")
	assert.Less(t, elapsed, 2*time.Second, "should not have hung beyond the budget")
}

func TestNATSKV_WithReplicas(t *testing.T) {
	nc := startEmbeddedNATS(t)

	tests := []struct {
		name     string
		replicas int
		wantRep  int
	}{
		{"no WithReplicas defaults to 1", 0, 1},
		{"explicit 1 replica", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket := sanitizeBucketName("test-replicas-" + tt.name)
			opts := []Option{
				WithTTL(5 * time.Second),
				WithNATS(nc, bucket),
			}
			if tt.replicas > 0 {
				opts = append(opts, WithReplicas(tt.replicas))
			}
			c, err := New[string](opts...)
			require.NoError(t, err)

			// Verify replica count via the backing stream config
			nkv := c.(*natsKVCache[string])
			js, err := jetstream.New(nc)
			require.NoError(t, err)
			stream, err := js.Stream(context.Background(), "KV_"+nkv.bucket)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRep, stream.CachedInfo().Config.Replicas)
		})
	}

	// Replicas > 1 requires a multi-node NATS cluster. Verify that the option
	// is actually passed through by confirming that a single-node server rejects it.
	t.Run("replicas 3 rejected by single-node server", func(t *testing.T) {
		_, err := New[string](
			WithTTL(5*time.Second),
			WithNATS(nc, "test-replicas-3"),
			WithReplicas(3),
		)
		require.Error(t, err, "single-node NATS should reject replicas > 1")
	})
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
