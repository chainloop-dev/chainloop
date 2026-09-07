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
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPubliclyRoutable(t *testing.T) {
	testCases := []struct {
		name string
		ip   string
		want bool
	}{
		{name: "public IPv4", ip: "8.8.8.8", want: true},
		{name: "public IPv6", ip: "2606:4700:4700::1111", want: true},

		{name: "IPv4 loopback", ip: "127.0.0.1", want: false},
		{name: "IPv6 loopback", ip: "::1", want: false},
		{name: "unspecified IPv4", ip: "0.0.0.0", want: false},
		{name: "unspecified IPv6", ip: "::", want: false},
		{name: "IPv4 broadcast", ip: "255.255.255.255", want: false},
		{name: "IPv4 multicast", ip: "224.0.0.1", want: false},

		{name: "RFC 1918 10/8", ip: "10.0.0.1", want: false},
		{name: "RFC 1918 172.16/12", ip: "172.16.0.1", want: false},
		{name: "RFC 1918 192.168/16", ip: "192.168.1.1", want: false},
		{name: "IPv6 unique local", ip: "fc00::1", want: false},

		// Cloud metadata services live in the link-local range
		{name: "AWS and Azure IMDS", ip: "169.254.169.254", want: false},
		{name: "IPv6 link-local", ip: "fe80::1", want: false},

		{name: "CGNAT shared address space", ip: "100.64.0.1", want: false},
		{name: "IETF protocol assignments", ip: "192.0.0.1", want: false},
		{name: "benchmarking range", ip: "198.18.0.1", want: false},
		{name: "reserved 240/4", ip: "240.0.0.1", want: false},
		{name: "this network 0/8", ip: "0.1.2.3", want: false},
		{name: "IPv4 documentation 192.0.2/24", ip: "192.0.2.1", want: false},
		{name: "IPv4 documentation 198.51.100/24", ip: "198.51.100.1", want: false},
		{name: "IPv4 documentation 203.0.113/24", ip: "203.0.113.1", want: false},
		{name: "IPv6 discard-only", ip: "100::1", want: false},
		{name: "IPv6 benchmarking", ip: "2001:2::1", want: false},
		{name: "IPv6 documentation", ip: "2001:db8::1", want: false},

		// An IPv4 address reached through an IPv6 form must be judged as the
		// IPv4 address it carries.
		{name: "IPv4-mapped loopback", ip: "::ffff:127.0.0.1", want: false},
		{name: "IPv4-mapped private", ip: "::ffff:10.0.0.1", want: false},
		{name: "IPv4-mapped IMDS", ip: "::ffff:169.254.169.254", want: false},
		{name: "IPv4-mapped public", ip: "::ffff:8.8.8.8", want: true},
		{name: "IPv4-compatible", ip: "::10.0.0.1", want: false},

		// IPv6 transition ranges embed an IPv4 address, so they are rejected
		// wholesale rather than unwrapped.
		{name: "NAT64", ip: "64:ff9b::a00:1", want: false},
		{name: "Teredo", ip: "2001::1", want: false},
		{name: "6to4", ip: "2002::1", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			require.NotNil(t, ip, "invalid test fixture %q", tc.ip)

			assert.Equal(t, tc.want, isPubliclyRoutable(ip))
		})
	}
}

func TestIsPubliclyRoutableRejectsNilIP(t *testing.T) {
	assert.False(t, isPubliclyRoutable(nil))
}

func TestPublicOnlyDialContext(t *testing.T) {
	testCases := []struct {
		name string
		// resolved is what DNS returns for the dialed host
		resolved []string
		// wantDialedTo is the address the underlying dialer must receive. The
		// client has to connect to the IP that was validated, not to the
		// hostname, so that a second DNS answer cannot change the target.
		wantDialedTo string
		wantBlocked  bool
	}{
		{
			name:         "public address is dialed by IP",
			resolved:     []string{"8.8.8.8"},
			wantDialedTo: "8.8.8.8:443",
		},
		{
			name:        "private address is blocked",
			resolved:    []string{"10.0.0.1"},
			wantBlocked: true,
		},
		{
			name:        "metadata endpoint is blocked",
			resolved:    []string{"169.254.169.254"},
			wantBlocked: true,
		},
		{
			name:        "blocked when any answer is non-public",
			resolved:    []string{"8.8.8.8", "127.0.0.1"},
			wantBlocked: true,
		},
		{
			name:         "first public answer is used",
			resolved:     []string{"1.1.1.1", "8.8.8.8"},
			wantDialedTo: "1.1.1.1:443",
		},
		{
			name:        "host without any answer is blocked",
			resolved:    []string{},
			wantBlocked: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolve := func(_ context.Context, _ string) ([]net.IPAddr, error) {
				addrs := make([]net.IPAddr, 0, len(tc.resolved))
				for _, ip := range tc.resolved {
					addrs = append(addrs, net.IPAddr{IP: net.ParseIP(ip)})
				}
				return addrs, nil
			}

			var dialedTo string
			dial := func(_ context.Context, _, addr string) (net.Conn, error) {
				dialedTo = addr
				return nil, nil
			}

			_, err := publicOnlyDialContext(resolve, dial)(context.Background(), "tcp", "example.com:443")

			if tc.wantBlocked {
				require.ErrorIs(t, err, ErrBlockedTarget)
				assert.Empty(t, dialedTo, "a blocked target must not be dialed")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantDialedTo, dialedTo)
		})
	}
}

func TestPublicOnlyDialContextResolutionFailure(t *testing.T) {
	resolve := func(_ context.Context, _ string) ([]net.IPAddr, error) {
		return nil, errors.New("no such host")
	}
	dial := func(_ context.Context, _, _ string) (net.Conn, error) {
		return nil, errors.New("dial must not be reached")
	}

	_, err := publicOnlyDialContext(resolve, dial)(context.Background(), "tcp", "example.com:443")
	require.Error(t, err)
}

func TestNewHTTPClientPublicTargetsOnly(t *testing.T) {
	// httptest listens on the loopback interface, so it stands in for any
	// destination a public-only client must refuse to reach.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	testCases := []struct {
		name              string
		publicTargetsOnly bool
		wantBlocked       bool
	}{
		{name: "public-only client refuses a loopback target", publicTargetsOnly: true, wantBlocked: true},
		{name: "unrestricted client reaches a loopback target", publicTargetsOnly: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewHTTPClient(HTTPClientOptions{PublicTargetsOnly: tc.publicTargetsOnly})

			resp, err := client.Get(server.URL)
			if tc.wantBlocked {
				require.ErrorIs(t, err, ErrBlockedTarget)
				return
			}

			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

// A proxy would be the only address a public-only client connects to, leaving
// the destination unchecked, so such a client must not pick one up from the
// environment. Asserted on the transport because net/http resolves the
// environment once per process, which a test cannot change after the fact.
func TestNewHTTPClientProxyUse(t *testing.T) {
	testCases := []struct {
		name              string
		publicTargetsOnly bool
		wantProxy         bool
	}{
		{name: "public-only client ignores an environment proxy", publicTargetsOnly: true},
		{name: "unrestricted client keeps an environment proxy", wantProxy: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewHTTPClient(HTTPClientOptions{PublicTargetsOnly: tc.publicTargetsOnly})

			transport, ok := client.Transport.(*http.Transport)
			require.True(t, ok)

			if tc.wantProxy {
				assert.NotNil(t, transport.Proxy)
				return
			}
			assert.Nil(t, transport.Proxy)
		})
	}
}

// Each redirect hop opens its own connection, so the dial-time check covers
// the whole chain: a first hop that passes cannot forward the client on to a
// blocked destination. Resolution is stubbed because every test server listens
// on the loopback interface, which no public-only client would reach at all.
func TestPublicOnlyDialContextValidatesEveryRedirectHop(t *testing.T) {
	var targetRequests int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetRequests++
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	// The URL the caller supplies resolves to a public address; the redirect
	// target it is then sent to resolves to a private one.
	var resolved int
	resolve := func(_ context.Context, _ string) ([]net.IPAddr, error) {
		resolved++
		if resolved == 1 {
			return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
		}
		return []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}}, nil
	}

	// The guard dials the address it validated, so point the dialer back at
	// the test server on that same port to keep the test off the network.
	dialer := &net.Dialer{}
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort("127.0.0.1", port))
	}

	client := &http.Client{Transport: &http.Transport{DialContext: publicOnlyDialContext(resolve, dial)}}

	_, err := client.Get(redirector.URL)
	require.ErrorIs(t, err, ErrBlockedTarget)
	assert.Equal(t, 2, resolved, "every hop must be resolved and validated")
	assert.Zero(t, targetRequests, "the redirect target must not be reached")
}
