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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	streamUpdateTimeout = 5 * time.Second
	// initMaxWait covers several nats.ReconnectWait (2s) cycles so a transient
	// drop during boot can heal before we give up.
	initMaxWait     = 30 * time.Second
	initRetryPeriod = 2 * time.Second
)

type natsKVCache[T any] struct {
	mu     sync.RWMutex
	kv     jetstream.KeyValue
	logger Logger
	conn   *nats.Conn
	bucket string
	cfg    *config
}

func newNATSKV[T any](cfg *config) (*natsKVCache[T], error) {
	c := &natsKVCache[T]{
		logger: cfg.logger,
		conn:   cfg.natsConn,
		bucket: cfg.bucketName,
		cfg:    cfg,
	}

	if err := c.initBucketWithRetry(initMaxWait, initRetryPeriod); err != nil {
		return nil, err
	}

	if cfg.reconnCh != nil {
		go c.watchReconnect(cfg.reconnCh)
	}

	cfg.logger.Infow("msg", "cache: using NATS KV backend", "bucket", cfg.bucketName, "ttl", cfg.ttl)
	return c, nil
}

// initBucketWithRetry runs initBucket with a bounded retry loop so transient
// NATS disconnects (handled by the client's background auto-reconnect) don't
// cause the whole service to refuse startup on a momentary blip. Retries only
// apply to connectivity errors; configuration errors fail fast.
func (c *natsKVCache[T]) initBucketWithRetry(maxWait, period time.Duration) error {
	deadline := time.Now().Add(maxWait)
	for attempt := 1; ; attempt++ {
		err := c.initBucket()
		if err == nil {
			return nil
		}
		if !isRetryableInitError(err) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("cache: bucket %q init failed after %s (%d attempts): %w", c.bucket, maxWait, attempt, err)
		}
		c.logger.Warnw("msg", "cache: bucket init failed, retrying", "bucket", c.bucket, "attempt", attempt, "error", err)
		time.Sleep(period)
	}
}

// isRetryableInitError reports whether err looks like a transient NATS
// connectivity issue worth retrying. Config errors (e.g. unsupported replica
// count) fail fast so misconfigured deployments don't spin for the full budget.
func isRetryableInitError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	if errors.Is(err, nats.ErrConnectionClosed) ||
		errors.Is(err, nats.ErrConnectionDraining) ||
		errors.Is(err, nats.ErrNoServers) ||
		errors.Is(err, nats.ErrTimeout) ||
		errors.Is(err, nats.ErrDisconnected) {
		return true
	}
	return false
}

func (c *natsKVCache[T]) initBucket() error {
	js, err := jetstream.New(c.conn)
	if err != nil {
		return err
	}

	kv, err := js.CreateOrUpdateKeyValue(context.Background(), jetstream.KeyValueConfig{
		Bucket:      c.bucket,
		Description: c.cfg.description,
		TTL:         c.cfg.ttl,
		MaxBytes:    c.cfg.maxBytes,
		Replicas:    c.cfg.replicas,
		Storage:     jetstream.MemoryStorage,
	})
	if err != nil {
		return err
	}

	// NATS KV hardcodes DiscardNew; we want DiscardOld so the cache evicts
	// oldest entries when MaxBytes is reached.
	if c.cfg.maxBytes > 0 {
		if err := c.ensureDiscardOld(js); err != nil {
			return err
		}
	}

	c.mu.Lock()
	c.kv = kv
	c.mu.Unlock()

	return nil
}

func (c *natsKVCache[T]) ensureDiscardOld(js jetstream.JetStream) error {
	streamName := fmt.Sprintf("KV_%s", c.bucket)

	ctx, cancel := context.WithTimeout(context.Background(), streamUpdateTimeout)
	defer cancel()

	stream, err := js.Stream(ctx, streamName)
	if err != nil {
		return fmt.Errorf("cache: failed to get backing stream %s: %w", streamName, err)
	}

	cfg := stream.CachedInfo().Config
	if cfg.Discard == jetstream.DiscardOld {
		return nil
	}
	cfg.Discard = jetstream.DiscardOld

	if _, err := js.UpdateStream(ctx, cfg); err != nil {
		return fmt.Errorf("cache: failed to set DiscardOld on stream %s: %w", streamName, err)
	}
	return nil
}

func (c *natsKVCache[T]) watchReconnect(ch <-chan struct{}) {
	for range ch {
		c.logger.Infow("msg", "cache: NATS reconnected, reinitializing bucket", "bucket", c.bucket)
		// Share the initial-boot retry budget: a reconnect may race with
		// NATS leader election / cluster settle, so one shot isn't enough.
		// Runtime ops already fail-open, so this is best-effort.
		if err := c.initBucketWithRetry(initMaxWait, initRetryPeriod); err != nil {
			c.logger.Warnw("msg", "cache: failed to reinitialize bucket after reconnect", "bucket", c.bucket, "error", err)
		}
	}
}

// sanitizeKey encodes the key for NATS KV compatibility.
// NATS KV keys only allow alphanumeric, '.', '-', '_', '/'.
// We use base64url encoding to avoid collisions from character replacement.
func sanitizeKey(key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

func (c *natsKVCache[T]) getKV() jetstream.KeyValue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.kv
}

// All operations degrade gracefully: if the KV handle is nil or a NATS
// operation fails, the method returns a cache miss / no-op instead of an error.
// This keeps the cache fail-open so callers fall through to the source of truth.

func (c *natsKVCache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	var zero T
	kv := c.getKV()
	if kv == nil {
		c.logger.Warnw("msg", "cache get: KV handle is nil, returning miss", "key", key, "backend", "nats")
		return zero, false, nil
	}

	sKey := sanitizeKey(key)
	entry, err := kv.Get(ctx, sKey)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			c.logger.Debugw("msg", "cache get", "key", key, "hit", false, "backend", "nats")
			return zero, false, nil
		}
		c.logger.Warnw("msg", "cache get error", "key", key, "error", err, "backend", "nats")
		return zero, false, nil
	}

	var val T
	if err := json.Unmarshal(entry.Value(), &val); err != nil {
		c.logger.Warnw("msg", "cache get: unmarshal failed, deleting corrupted entry", "key", key, "error", err, "backend", "nats")
		_ = kv.Delete(ctx, sKey)
		return zero, false, nil
	}

	c.logger.Debugw("msg", "cache get", "key", key, "hit", true, "backend", "nats")
	return val, true, nil
}

func (c *natsKVCache[T]) Set(ctx context.Context, key string, value T) error {
	kv := c.getKV()
	if kv == nil {
		c.logger.Warnw("msg", "cache set: KV handle is nil, skipping", "key", key, "backend", "nats")
		return nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if _, err := kv.Put(ctx, sanitizeKey(key), data); err != nil {
		c.logger.Warnw("msg", "cache set error", "key", key, "error", err, "backend", "nats")
		return nil
	}

	c.logger.Debugw("msg", "cache set", "key", key, "backend", "nats")
	return nil
}

func (c *natsKVCache[T]) Delete(ctx context.Context, key string) error {
	kv := c.getKV()
	if kv == nil {
		return nil
	}

	if err := kv.Delete(ctx, sanitizeKey(key)); err != nil {
		if !errors.Is(err, jetstream.ErrKeyNotFound) {
			c.logger.Warnw("msg", "cache delete error", "key", key, "error", err, "backend", "nats")
		}
	}

	c.logger.Debugw("msg", "cache delete", "key", key, "backend", "nats")
	return nil
}

func (c *natsKVCache[T]) Purge(ctx context.Context) error {
	kv := c.getKV()
	if kv == nil {
		return nil
	}

	keys, err := kv.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil
		}
		c.logger.Warnw("msg", "cache purge: failed to list keys", "error", err, "backend", "nats")
		return nil
	}

	for _, k := range keys {
		if err := kv.Purge(ctx, k); err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
			c.logger.Warnw("msg", "cache purge: failed to purge key", "key", k, "error", err, "backend", "nats")
		}
	}

	c.logger.Debugw("msg", "cache purge", "backend", "nats")
	return nil
}
