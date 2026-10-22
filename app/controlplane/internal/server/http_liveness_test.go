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

package server

import (
	"context"
	"testing"
	"time"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSharedServerConnectionLiveness verifies the shared control-plane endpoint
// is built with the connection-liveness limits — an idle timeout, HTTP/2
// keepalive pings, per-connection stream and frame bounds, and a connection cap
// — on the h2c-enabled public listener (PFM-7981). net/http keeps an idle HTTP/2
// connection open indefinitely without these, since ReadHeaderTimeout does not
// apply on the HTTP/2 path.
func TestSharedServerConnectionLiveness(t *testing.T) {
	opts := testServerOpts(t, &conf.Server{
		Grpc: &conf.Server_GRPC{Addr: testAnyAddr},
		Http: &conf.Server_HTTP{Addr: testAnyAddr},
	})
	gs, err := NewGRPCServers(opts)
	require.NoError(t, err)
	api, err := NewHTTPServer(opts, gs)
	require.NoError(t, err)

	// Start and stop the server so the bound listener is released cleanly.
	errCh := make(chan error, 1)
	go func() { errCh <- api.Start(context.Background()) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, api.Stop(ctx))
		require.NoError(t, <-errCh)
	})

	require.NotNil(t, api.HTTP2, "the shared HTTP/2 config must be set so keepalive/liveness limits apply")
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"IdleTimeout", api.IdleTimeout, publicIdleTimeout},
		{"HTTP2.SendPingTimeout", api.HTTP2.SendPingTimeout, publicHTTP2PingInterval},
		{"HTTP2.PingTimeout", api.HTTP2.PingTimeout, publicHTTP2PingTimeout},
		{"HTTP2.MaxReadFrameSize", api.HTTP2.MaxReadFrameSize, publicHTTP2MaxReadFrameSize},
		{"HTTP2.MaxConcurrentStreams", api.HTTP2.MaxConcurrentStreams, publicHTTP2MaxConcurrentStreams},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.got)
		})
	}

	// Sanity: the production value must stay generous enough never to trip on
	// legitimate traffic. A misconfigured tiny cap would reject valid clients.
	assert.GreaterOrEqual(t, maxPublicConns, 1024, "the connection cap must stay well above realistic concurrency")
	assert.Equal(t, 120*time.Second, publicIdleTimeout)
}
