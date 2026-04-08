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
	"errors"
	"time"

	"github.com/nats-io/nats.go"
)

// Cache is a generic cache interface with TTL-based expiration.
// TTL is configured at construction time, not per-operation.
type Cache[T any] interface {
	Get(ctx context.Context, key string) (T, bool, error)
	Set(ctx context.Context, key string, value T) error
	Delete(ctx context.Context, key string) error
	Purge(ctx context.Context) error
}

// Logger is a structured logging interface satisfied by *log.Helper from Kratos.
type Logger interface {
	Debugw(keyvals ...any)
	Warnw(keyvals ...any)
	Infow(keyvals ...any)
	Errorw(keyvals ...any)
}

// defaultMaxSize is a sensible upper bound on in-memory cache entries
// to prevent unbounded growth. 0 means no LRU eviction (TTL-only).
const defaultMaxSize = 1000

type config struct {
	ttl         time.Duration
	maxSize     int
	logger      Logger
	natsConn    *nats.Conn
	bucketName  string
	description string
	reconnCh    <-chan struct{}
}

// Option configures cache construction.
type Option func(*config)

// WithTTL sets the expiration duration for cache entries. Required.
func WithTTL(d time.Duration) Option {
	return func(c *config) { c.ttl = d }
}

// WithLogger sets a structured logger for cache operations.
func WithLogger(l Logger) Option {
	return func(c *config) { c.logger = l }
}

// WithNATS enables the NATS JetStream KV backend.
func WithNATS(conn *nats.Conn, bucketName string) Option {
	return func(c *config) {
		c.natsConn = conn
		c.bucketName = bucketName
	}
}

// WithDescription sets the NATS KV bucket description. Ignored for in-memory backend.
func WithDescription(desc string) Option {
	return func(c *config) { c.description = desc }
}

// WithReconnect provides a channel that signals NATS reconnection events.
func WithReconnect(ch <-chan struct{}) Option {
	return func(c *config) { c.reconnCh = ch }
}

// New creates a new Cache[T]. If NATS options are provided and the connection
// is non-nil, a NATS KV backend is returned. Otherwise an in-memory LRU is used.
func New[T any](opts ...Option) (Cache[T], error) {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.ttl <= 0 {
		return nil, errors.New("cache: TTL must be greater than 0")
	}

	if cfg.logger == nil {
		cfg.logger = nopLogger{}
	}

	if cfg.natsConn != nil {
		if cfg.bucketName == "" {
			return nil, errors.New("cache: bucket name is required when NATS backend is enabled")
		}
		return newNATSKV[T](cfg)
	}

	if cfg.maxSize == 0 {
		cfg.maxSize = defaultMaxSize
	}

	return newMemoryCache[T](cfg), nil
}

// nopLogger is a no-op implementation of Logger.
type nopLogger struct{}

func (nopLogger) Debugw(_ ...any) {}
func (nopLogger) Warnw(_ ...any)  {}
func (nopLogger) Infow(_ ...any)  {}
func (nopLogger) Errorw(_ ...any) {}
