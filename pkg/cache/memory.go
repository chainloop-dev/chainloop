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

	"github.com/hashicorp/golang-lru/v2/expirable"
)

type memoryCache[T any] struct {
	lru    *expirable.LRU[string, T]
	logger Logger
}

func newMemoryCache[T any](cfg *config) *memoryCache[T] {
	cfg.logger.Infow("cache: using in-memory LRU backend", "ttl", cfg.ttl, "maxSize", cfg.maxSize)
	return &memoryCache[T]{
		lru:    expirable.NewLRU[string, T](cfg.maxSize, nil, cfg.ttl),
		logger: cfg.logger,
	}
}

func (m *memoryCache[T]) Get(_ context.Context, key string) (T, bool, error) {
	val, ok := m.lru.Get(key)
	m.logger.Debugw("cache get", "key", key, "hit", ok, "backend", "memory")
	return val, ok, nil
}

func (m *memoryCache[T]) Set(_ context.Context, key string, value T) error {
	m.lru.Add(key, value)
	m.logger.Debugw("cache set", "key", key, "backend", "memory")
	return nil
}

func (m *memoryCache[T]) Delete(_ context.Context, key string) error {
	m.lru.Remove(key)
	m.logger.Debugw("cache delete", "key", key, "backend", "memory")
	return nil
}

func (m *memoryCache[T]) Purge(_ context.Context) error {
	m.lru.Purge()
	m.logger.Debugw("cache purge", "backend", "memory")
	return nil
}
