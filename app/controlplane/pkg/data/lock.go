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

package data

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/go-kratos/kratos/v2/log"
)

// Cap on how long the release path may block. Defends against a stuck
// session: if pg_advisory_unlock can't return, we drop the connection
// and let Postgres release the lock on session disconnect.
const advisoryUnlockTimeout = 5 * time.Second

// PostgresLock implements biz.DistributedLock using Postgres session-level
// advisory locks (pg_try_advisory_lock / pg_advisory_unlock).
//
// Postgres is the only piece of infrastructure that's mandatory for the
// control plane — NATS is optional (used for distributed caches when
// present). Using advisory locks lets us coordinate background jobs
// across replicas without adding a new dependency (a NATS KV lease or a
// dedicated queue) that wouldn't be available in every deployment.
//
// Each lock holds a dedicated connection for its lifetime; releasing the
// lock returns the connection to the pool. If the pod crashes mid-run the
// connection drops and Postgres releases the lock automatically.
type PostgresLock struct {
	db  *sql.DB
	log *log.Helper
}

func NewPostgresLock(d *Data, logger log.Logger) biz.DistributedLock {
	return &PostgresLock{
		db:  d.SQLDB,
		log: log.NewHelper(logger),
	}
}

func (l *PostgresLock) TryAcquire(ctx context.Context, key string) (bool, func(), error) {
	intKey := hashKey(key)

	conn, err := l.db.Conn(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("acquiring DB connection: %w", err)
	}

	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", intKey).Scan(&acquired); err != nil {
		_ = conn.Close()
		return false, nil, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}

	if !acquired {
		_ = conn.Close()
		return false, nil, nil
	}

	release := func() {
		// pg_advisory_unlock must run on the same session that took the lock,
		// and must run even if the caller's context was cancelled (e.g. shutdown).
		// Bounded so a stuck session can't hang the release path.
		releaseCtx, cancel := context.WithTimeout(context.Background(), advisoryUnlockTimeout)
		defer cancel()
		if _, err := conn.ExecContext(releaseCtx, "SELECT pg_advisory_unlock($1)", intKey); err != nil {
			l.log.Warnw("msg", "failed to release advisory lock", "key", key, "error", err)
		}
		if err := conn.Close(); err != nil {
			l.log.Warnw("msg", "failed to return DB connection to pool", "key", key, "error", err)
		}
	}
	return true, release, nil
}

// hashKey turns an opaque string key into the int64 that
// pg_advisory_lock expects. FNV-1a gives a stable, well-distributed
// mapping with effectively no collision risk for the handful of named
// locks this package uses.
func hashKey(key string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return int64(h.Sum64()) //nolint:gosec // intentional wraparound; pg accepts any int64
}
