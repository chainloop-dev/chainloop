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

// Package netguard restricts outbound HTTP connections to publicly routable
// destinations.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// ErrBlockedTarget is returned when a request is refused because its
// destination is not publicly routable.
var ErrBlockedTarget = errors.New("blocked outbound request")

const (
	dialTimeout   = 10 * time.Second
	dialKeepAlive = 30 * time.Second
)

// ResolveFunc resolves a host name into the addresses it points to.
type ResolveFunc func(ctx context.Context, host string) ([]net.IPAddr, error)

// DialFunc opens a connection to addr, in the form of net.Dialer.DialContext.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// RestrictTransport makes t connect to publicly routable destinations only:
// loopback, private ranges, link-local addresses (where cloud metadata
// services live) and the IPv6 transition ranges that embed an IPv4 address are
// all refused with ErrBlockedTarget.
//
// It also drops any proxy, which would otherwise be the only address the
// transport connects to and would leave the destination unchecked.
//
// t is modified in place and returned. A nil t starts from a clone of
// http.DefaultTransport.
func RestrictTransport(t *http.Transport) *http.Transport {
	if t == nil {
		t = http.DefaultTransport.(*http.Transport).Clone()
	}

	// The transport's own dialer is replaced rather than wrapped, since it may
	// ignore the address it is given and connect somewhere else entirely.
	dialer := &net.Dialer{Timeout: dialTimeout, KeepAlive: dialKeepAlive}
	t.DialContext = PublicOnlyDialContext(net.DefaultResolver.LookupIPAddr, dialer.DialContext)
	t.DialTLSContext = nil
	t.Proxy = nil

	return t
}

// Transport returns a process-wide transport that only connects to publicly
// routable destinations, so that its connections are pooled and reused across
// requests. See RestrictTransport.
var Transport = sync.OnceValue(func() *http.Transport {
	return RestrictTransport(nil)
})

// NewHTTPClient returns a client that only connects to publicly routable
// destinations, backed by Transport.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: Transport()}
}

// PublicOnlyDialContext wraps dial so that a connection is only made to a
// publicly routable address.
//
// The check runs here, at dial time, rather than against the URL, for two
// reasons. It sees the address the connection will actually use, so a host
// name that resolves to an allowed address for a check and to a blocked one
// for the connection cannot slip through: the dial targets the very IP that
// was validated. And because every redirect hop opens its own connection,
// the whole chain is covered, not just the URL the caller supplied.
func PublicOnlyDialContext(resolve ResolveFunc, dial DialFunc) DialFunc {
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
			if !IsPubliclyRoutable(ip.IP) {
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

// IsPubliclyRoutable reports whether ip is an address on the public internet.
func IsPubliclyRoutable(ip net.IP) bool {
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
		panic(fmt.Sprintf("netguard: invalid CIDR %q: %v", cidr, err))
	}

	return network
}
