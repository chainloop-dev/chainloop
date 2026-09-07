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
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// ErrBlockedTarget is returned when a request is refused because its
// destination is not allowed by the client's network policy.
var ErrBlockedTarget = errors.New("blocked outbound request")

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
		transport.DialContext = publicOnlyDialContext(net.DefaultResolver.LookupIPAddr, dialer.DialContext)

		// Through a proxy the only address this client connects to is the
		// proxy's own, which leaves the destination unchecked and the guard
		// above unenforced, so a public-only client never uses one.
		transport.Proxy = nil
	}

	return &http.Client{Timeout: opts.Timeout, Transport: transport}
}

type resolveFunc func(ctx context.Context, host string) ([]net.IPAddr, error)

type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// publicOnlyDialContext wraps dial so that a connection is only made to a
// publicly routable address.
//
// The check runs here, at dial time, rather than against the URL, for two
// reasons. It sees the address the connection will actually use, so a host
// name that resolves to an allowed address for a check and to a blocked one
// for the connection cannot slip through: the dial targets the very IP that
// was validated. And because every redirect hop opens its own connection,
// the whole chain is covered, not just the URL the caller supplied.
func publicOnlyDialContext(resolve resolveFunc, dial dialFunc) dialFunc {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("%w: malformed address %q", ErrBlockedTarget, addr)
		}

		ips, err := resolve(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolving %q: %w", host, err)
		}

		if len(ips) == 0 {
			return nil, fmt.Errorf("%w: %q did not resolve to any address", ErrBlockedTarget, host)
		}

		// A host that answers with a mix of public and non-public addresses is
		// refused outright, so that retrying cannot land on the blocked one.
		for _, ip := range ips {
			if !isPubliclyRoutable(ip.IP) {
				return nil, fmt.Errorf("%w: %q resolves to non-public address %s", ErrBlockedTarget, host, ip.IP)
			}
		}

		var lastErr error
		for _, ip := range ips {
			conn, err := dial(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}

		return nil, lastErr
	}
}

// blockedNets holds the ranges that are not publicly routable but that the
// checks net.IP offers do not already cover.
var blockedNets = []*net.IPNet{
	mustParseCIDR("0.0.0.0/8"),       // RFC 6890 "this network"
	mustParseCIDR("100.64.0.0/10"),   // RFC 6598 shared address space (CGNAT)
	mustParseCIDR("192.0.0.0/24"),    // RFC 6890 IETF protocol assignments
	mustParseCIDR("192.0.2.0/24"),    // RFC 5737 documentation
	mustParseCIDR("198.18.0.0/15"),   // RFC 2544 benchmarking
	mustParseCIDR("198.51.100.0/24"), // RFC 5737 documentation
	mustParseCIDR("203.0.113.0/24"),  // RFC 5737 documentation
	mustParseCIDR("240.0.0.0/4"),     // RFC 1112 reserved
	mustParseCIDR("::/96"),           // RFC 4291 IPv4-compatible, deprecated
	mustParseCIDR("64:ff9b::/96"),    // RFC 6052 NAT64
	mustParseCIDR("100::/64"),        // RFC 6666 discard-only
	mustParseCIDR("2001::/32"),       // RFC 4380 Teredo
	mustParseCIDR("2001:2::/48"),     // RFC 5180 benchmarking
	mustParseCIDR("2001:db8::/32"),   // RFC 3849 documentation
	mustParseCIDR("2002::/16"),       // RFC 3056 6to4
}

// isPubliclyRoutable reports whether ip is an address on the public internet.
func isPubliclyRoutable(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Covers the unspecified address, loopback, link-local addresses (and with
	// them the cloud metadata endpoints), multicast and the IPv4 broadcast
	// address.
	if !ip.IsGlobalUnicast() {
		return false
	}

	// RFC 1918 ranges and RFC 4193 unique local addresses.
	if ip.IsPrivate() {
		return false
	}

	// An IPv4 address reached through an IPv6 form is judged as the IPv4
	// address it carries. The checks above already do this; the ranges below
	// are written in their IPv4 form, so narrow the address first.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}

	for _, blocked := range blockedNets {
		if blocked.Contains(ip) {
			return false
		}
	}

	return true
}

func mustParseCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(fmt.Sprintf("sdk: invalid CIDR %q: %v", cidr, err))
	}

	return network
}
