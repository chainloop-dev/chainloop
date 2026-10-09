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

package sdk

import (
	"net"
	"net/http"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/netguard"
)

// ErrBlockedTarget is returned when a request is refused because its
// destination is not allowed by the client's network policy.
var ErrBlockedTarget = netguard.ErrBlockedTarget

const (
	dialTimeout      = 10 * time.Second
	dialKeepAlive    = 30 * time.Second
	tlsHandshakeWait = 10 * time.Second
	idleConnTimeout  = 90 * time.Second
)

// NetworkPolicy carries the outbound restrictions a deployment puts on the
// plugins whose destination is an arbitrary URL taken from their registration
// config. It mirrors the control plane's plugins_network_policy config.
type NetworkPolicy struct {
	// BlockPrivateTargets refuses destinations that are not publicly
	// routable. It is off by default, because a deployment may well run the
	// service a plugin talks to inside its own network.
	BlockPrivateTargets bool
}

// HTTPClientOptions configures the client returned by NewHTTPClient.
type HTTPClientOptions struct {
	// Timeout caps the total duration of a single request, including
	// redirects and reading the response body. Zero leaves the client
	// without a timeout.
	Timeout time.Duration

	// PublicTargetsOnly refuses to connect to destinations that are not
	// publicly routable: loopback, private ranges, link-local addresses
	// (where cloud metadata services live) and the IPv6 transition ranges
	// that embed an IPv4 address.
	//
	// Such a client also ignores any proxy configured in the environment,
	// which would otherwise be the only address it connects to and would
	// leave the destination unchecked.
	//
	// Enable it for plugins whose destination is a well-known public service.
	// Plugins that legitimately talk to hosts inside the deployment's own
	// network must leave it disabled.
	PublicTargetsOnly bool
}

// NewHTTPClient builds the HTTP client a plugin should use for requests to a
// destination taken from its registration config.
func NewHTTPClient(opts HTTPClientOptions) *http.Client {
	dialer := &net.Dialer{Timeout: dialTimeout, KeepAlive: dialKeepAlive}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeWait,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if opts.PublicTargetsOnly {
		netguard.RestrictTransport(transport)
	}

	return &http.Client{Timeout: opts.Timeout, Transport: transport}
}
