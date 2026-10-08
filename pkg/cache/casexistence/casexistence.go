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

// Package casexistence provides a typed cache of the blobs known to exist in
// a CAS backend, so repeated uploads of the same content skip the backend.
package casexistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/cache"
	"github.com/chainloop-dev/chainloop/pkg/natsconn"
	"github.com/go-kratos/kratos/v2/log"
)

const (
	// ttl bounds how long an entry outlives a purge of the blob outside Chainloop
	ttl         = 24 * time.Hour
	maxBytes    = 64 * 1024 * 1024 // 64 MB
	maxEntries  = 50_000
	bucket      = "chainloop-cas-existence"
	description = "Cache of blobs known to exist in a CAS backend"
)

// Entry is a blob known to exist in a backend.
type Entry struct {
	Size int64 `json:"size"`
}

// Cache wraps cache.Cache[Entry] to provide a distinct type for wire disambiguation.
type Cache struct {
	cache.Cache[Entry]
}

// New creates an existence cache, backed by NATS KV when rc is set and by an
// in-memory LRU otherwise. A NATS bucket that can not be set up falls back to
// memory: the cache only makes uploads faster, so it must not stop its caller
// from starting.
func New(ctx context.Context, rc *natsconn.ReloadableConnection, logger log.Logger) (*Cache, error) {
	c, err := newCache(ctx, rc, logger)
	if err != nil && rc != nil {
		if logger != nil {
			log.NewHelper(logger).Warnw("msg", "existence cache: NATS KV unavailable, using memory", "error", err)
		}
		c, err = newCache(ctx, nil, logger)
	}

	return c, err
}

func newCache(ctx context.Context, rc *natsconn.ReloadableConnection, logger log.Logger) (*Cache, error) {
	opts := []cache.Option{
		cache.WithTTL(ttl),
		cache.WithMaxBytes(maxBytes),
		cache.WithMaxEntries(maxEntries),
		cache.WithDescription(description),
	}

	if logger != nil {
		opts = append(opts, cache.WithLogger(log.NewHelper(logger)))
	}

	if rc != nil {
		opts = append(opts, cache.WithNATS(rc.Conn, bucket))
		opts = append(opts, cache.WithReconnect(rc.Subscribe(ctx)))
		opts = append(opts, cache.WithReplicas(rc.Replicas))
	}

	c, err := cache.New[Entry](opts...)
	if err != nil {
		return nil, err
	}
	return &Cache{Cache: c}, nil
}

// Key returns the cache key of a digest in the namespace of one organization
// and one backend. The secret reference is hashed so its path never appears in
// the shared store.
func Key(orgID, backendType, secretID, digest string) string {
	secretHash := sha256.Sum256([]byte(secretID))
	return strings.Join([]string{orgID, backendType, hex.EncodeToString(secretHash[:]), digest}, "/")
}
