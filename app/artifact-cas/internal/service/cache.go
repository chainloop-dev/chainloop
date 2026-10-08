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

package service

import (
	"context"
	"strings"
	"time"

	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/cache"
	"github.com/chainloop-dev/chainloop/pkg/cache/casexistence"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// maxCachedClients bounds the number of backend clients kept in memory
const maxCachedClients = 1000

var (
	existenceCacheLookups = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chainloop_cas_existence_cache_lookups_total",
		Help: "Lookups in the cache of blobs known to exist in a backend, by result (hit or miss).",
	}, []string{"result"})

	clientCacheLookups = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chainloop_cas_backend_client_cache_lookups_total",
		Help: "Lookups in the cache of loaded backend clients, by result (hit or miss).",
	}, []string{"result"})
)

func recordLookup(c *prometheus.CounterVec, hit bool) {
	if hit {
		c.WithLabelValues("hit").Inc()
	} else {
		c.WithLabelValues("miss").Inc()
	}
}

// WithExistenceCache enables the cache of blobs known to exist in a backend.
// Uploads of a cached digest end as "already exists" with no backend call.
func WithExistenceCache(c cache.Cache[casexistence.Entry]) NewOpt {
	return func(s *commonService) {
		s.existence = c
	}
}

// WithBackendClientCache enables the reuse of the backend clients loaded for
// uploads for ttl. Clients and their credentials are kept in process memory only.
// The services built with the same option share one cache.
func WithBackendClientCache(ttl time.Duration) NewOpt {
	clients := expirable.NewLRU[string, backend.UploaderDownloader](maxCachedClients, nil, ttl)
	return func(s *commonService) {
		s.clients = clients
	}
}

// existenceKey returns the existence cache key of digest in the namespace of
// the organization and backend of the token, or "" when the upload can not be cached.
func (s *commonService) existenceKey(info *casJWT.Claims, digest string) string {
	if s.existence == nil || info.OrgID == "" {
		return ""
	}

	return casexistence.Key(info.OrgID, info.BackendType, info.StoredSecretID, digest)
}

// lookupExistence reports if key is cached as present. Cache errors are a miss.
func (s *commonService) lookupExistence(ctx context.Context, key string) (casexistence.Entry, bool) {
	if key == "" {
		return casexistence.Entry{}, false
	}

	entry, ok, err := s.existence.Get(ctx, key)
	if err != nil {
		s.log.Warnw("msg", "existence cache lookup failed", "error", err)
		ok = false
	}

	recordLookup(existenceCacheLookups, ok)
	return entry, ok
}

// storeExistence caches that a blob of size is present. Best-effort.
func (s *commonService) storeExistence(ctx context.Context, key string, size int64) {
	if key == "" {
		return
	}

	if err := s.existence.Set(ctx, key, casexistence.Entry{Size: size}); err != nil {
		s.log.Warnw("msg", "existence cache store failed", "error", err)
	}
}

// clientKey identifies a backend client. It includes the organization because
// some backends (e.g. AWS-S3-ACCESS-POINT) keep per-organization session
// credentials inside the client.
func clientKey(info *casJWT.Claims) string {
	return strings.Join([]string{info.BackendType, info.OrgID, info.StoredSecretID}, "\x00")
}

// loadUploadBackend returns the backend of the token, reusing a cached client
// when the client cache is enabled. cached reports if the client was reused.
func (s *commonService) loadUploadBackend(ctx context.Context, info *casJWT.Claims) (b backend.UploaderDownloader, cached bool, err error) {
	if s.clients == nil {
		b, err = s.loadBackend(ctx, info.BackendType, info.StoredSecretID)
		return b, false, err
	}

	key := clientKey(info)
	if b, ok := s.clients.Get(key); ok {
		recordLookup(clientCacheLookups, true)
		return b, true, nil
	}
	recordLookup(clientCacheLookups, false)

	b, err = s.loadBackend(ctx, info.BackendType, info.StoredSecretID)
	if err != nil {
		return nil, false, err
	}

	s.clients.Add(key, b)
	return b, false, nil
}

// dropUploadBackend removes the cached client of the token, so the next upload
// loads the credentials again.
func (s *commonService) dropUploadBackend(info *casJWT.Claims) {
	if s.clients != nil {
		s.clients.Remove(clientKey(info))
	}
}
