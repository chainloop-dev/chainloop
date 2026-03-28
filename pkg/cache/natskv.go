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
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
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

	if err := c.initBucket(); err != nil {
		return nil, err
	}

	if cfg.reconnCh != nil {
		go c.watchReconnect(cfg.reconnCh)
	}

	cfg.logger.Infow("cache: using NATS KV backend", "bucket", cfg.bucketName, "ttl", cfg.ttl)
	return c, nil
}

func (c *natsKVCache[T]) initBucket() error {
	js, err := jetstream.New(c.conn)
	if err != nil {
		return err
	}

	kv, err := js.CreateOrUpdateKeyValue(context.Background(), jetstream.KeyValueConfig{
		Bucket:  c.bucket,
		TTL:     c.cfg.ttl,
		Storage: jetstream.MemoryStorage,
	})
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.kv = kv
	c.mu.Unlock()
	return nil
}

func (c *natsKVCache[T]) watchReconnect(ch <-chan struct{}) {
	for range ch {
		c.logger.Infow("cache: NATS reconnected, reinitializing bucket", "bucket", c.bucket)
		if err := c.initBucket(); err != nil {
			c.logger.Warnw("cache: failed to reinitialize bucket after reconnect", "bucket", c.bucket, "error", err)
		}
	}
}

// sanitizeKey replaces colons with dots for NATS subject token compatibility.
func sanitizeKey(key string) string {
	return strings.ReplaceAll(key, ":", ".")
}

func (c *natsKVCache[T]) getKV() jetstream.KeyValue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.kv
}

func (c *natsKVCache[T]) Get(_ context.Context, key string) (T, bool, error) {
	var zero T
	kv := c.getKV()
	if kv == nil {
		c.logger.Warnw("cache get: KV handle is nil, returning miss", "key", key, "backend", "nats")
		return zero, false, nil
	}

	sKey := sanitizeKey(key)
	entry, err := kv.Get(context.Background(), sKey)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			c.logger.Debugw("cache get", "key", key, "hit", false, "backend", "nats")
			return zero, false, nil
		}
		c.logger.Warnw("cache get error", "key", key, "error", err, "backend", "nats")
		return zero, false, nil
	}

	var val T
	if err := json.Unmarshal(entry.Value(), &val); err != nil {
		c.logger.Warnw("cache get: unmarshal failed, deleting corrupted entry", "key", key, "error", err, "backend", "nats")
		_ = kv.Delete(context.Background(), sKey)
		return zero, false, nil
	}

	c.logger.Debugw("cache get", "key", key, "hit", true, "backend", "nats")
	return val, true, nil
}

func (c *natsKVCache[T]) Set(_ context.Context, key string, value T) error {
	kv := c.getKV()
	if kv == nil {
		c.logger.Warnw("cache set: KV handle is nil, skipping", "key", key, "backend", "nats")
		return nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if _, err := kv.Put(context.Background(), sanitizeKey(key), data); err != nil {
		c.logger.Warnw("cache set error", "key", key, "error", err, "backend", "nats")
		return nil
	}

	c.logger.Debugw("cache set", "key", key, "backend", "nats")
	return nil
}

func (c *natsKVCache[T]) Delete(_ context.Context, key string) error {
	kv := c.getKV()
	if kv == nil {
		return nil
	}

	if err := kv.Delete(context.Background(), sanitizeKey(key)); err != nil {
		if !errors.Is(err, jetstream.ErrKeyNotFound) {
			c.logger.Warnw("cache delete error", "key", key, "error", err, "backend", "nats")
		}
	}

	c.logger.Debugw("cache delete", "key", key, "backend", "nats")
	return nil
}

func (c *natsKVCache[T]) Purge(_ context.Context) error {
	kv := c.getKV()
	if kv == nil {
		return nil
	}

	keys, err := kv.Keys(context.Background())
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil
		}
		c.logger.Warnw("cache purge: failed to list keys", "error", err, "backend", "nats")
		return nil
	}

	for _, k := range keys {
		_ = kv.Delete(context.Background(), k)
	}

	c.logger.Debugw("cache purge", "backend", "nats")
	return nil
}
