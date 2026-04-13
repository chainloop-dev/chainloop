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

// Package policyevalbundle provides a typed cache for policy evaluation bundles.
package policyevalbundle

import (
	"context"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/cache"
	"github.com/chainloop-dev/chainloop/pkg/natsconn"
	"github.com/go-kratos/kratos/v2/log"
)

const (
	ttl         = 24 * time.Hour
	maxBytes    = 100 * 1024 * 1024 // 100 MB
	bucket      = "chainloop-policy-eval-bundles"
	description = "Cache for policy evaluation bundles from CAS"
)

// Cache wraps cache.Cache[[]byte] to provide a distinct type for wire disambiguation.
type Cache struct {
	cache.Cache[[]byte]
}

// New creates a policy evaluation bundle cache with built-in TTL, bucket, and description.
func New(ctx context.Context, rc *natsconn.ReloadableConnection, logger log.Logger) (*Cache, error) {
	opts := []cache.Option{
		cache.WithTTL(ttl),
		cache.WithMaxBytes(maxBytes),
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

	c, err := cache.New[[]byte](opts...)
	if err != nil {
		return nil, err
	}
	return &Cache{Cache: c}, nil
}
