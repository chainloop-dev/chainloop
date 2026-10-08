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

package casexistence

import (
	"context"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/natsconn"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startNATS(t *testing.T, jetStream bool) *nats.Conn {
	t.Helper()
	ns, err := natsserver.NewServer(&natsserver.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: jetStream,
		StoreDir:  t.TempDir(),
	})
	require.NoError(t, err)
	ns.Start()
	t.Cleanup(ns.Shutdown)

	require.True(t, ns.ReadyForConnections(5*time.Second))
	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	return nc
}

// TestNew: the cache uses NATS KV when it can, and falls back to memory when
// the bucket can not be set up, so its caller still starts.
func TestNew(t *testing.T) {
	testCases := []struct {
		name       string
		jetStream  bool
		wantBucket bool
	}{
		{name: "NATS KV when JetStream is available", jetStream: true, wantBucket: true},
		{name: "memory when the bucket can not be set up", jetStream: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			nc := startNATS(t, tc.jetStream)

			c, err := New(ctx, &natsconn.ReloadableConnection{Conn: nc}, nil)
			require.NoError(t, err)

			require.NoError(t, c.Set(ctx, "key", Entry{Size: 42}))
			got, ok, err := c.Get(ctx, "key")
			require.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, int64(42), got.Size)

			if tc.wantBucket {
				js, err := jetstream.New(nc)
				require.NoError(t, err)
				_, err = js.KeyValue(context.Background(), bucket)
				assert.NoError(t, err, "the cache must be stored in its own NATS bucket")
			}
		})
	}
}
