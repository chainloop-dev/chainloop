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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSharedServerConnectionLiveness verifies the shared CAS endpoint is built
// with the connection-liveness limits — an idle timeout, HTTP/2 keepalive pings,
// per-connection stream and frame bounds, and a connection cap — on the
// h2c-enabled public listener (PFM-7981). net/http keeps an idle HTTP/2
// connection open indefinitely without these, since ReadHeaderTimeout does not
// apply on the HTTP/2 path. No absolute read/write deadline is set, so multi-GB
// uploads and downloads remain unaffected.
func TestSharedServerConnectionLiveness(t *testing.T) {
	opts := casTestOpts(t)
	servers, err := NewGRPCServers(opts)
	require.NoError(t, err)
	api, err := NewHTTPServer(opts, servers)
	require.NoError(t, err)
	startCASTestTransport(t, api)

	require.NotNil(t, api.httpSrv.HTTP2, "the shared HTTP/2 config must be set so keepalive/liveness limits apply")
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"IdleTimeout", api.httpSrv.IdleTimeout, publicIdleTimeout},
		{"HTTP2.SendPingTimeout", api.httpSrv.HTTP2.SendPingTimeout, publicHTTP2PingInterval},
		{"HTTP2.PingTimeout", api.httpSrv.HTTP2.PingTimeout, publicHTTP2PingTimeout},
		{"HTTP2.MaxReadFrameSize", api.httpSrv.HTTP2.MaxReadFrameSize, publicHTTP2MaxReadFrameSize},
		{"HTTP2.MaxConcurrentStreams", api.httpSrv.HTTP2.MaxConcurrentStreams, publicHTTP2MaxConcurrentStreams},
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
