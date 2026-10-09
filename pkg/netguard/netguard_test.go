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

package netguard

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	publicIP = "8.8.8.8"
	// Cloud metadata services listen on this link-local address
	metadataIP = "169.254.169.254"
)

func TestIsPubliclyRoutable(t *testing.T) {
	testCases := []struct {
		name string
		ip   string
		want bool
	}{
		{name: "public IPv4", ip: publicIP, want: true},
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
		{name: "AWS and Azure IMDS", ip: metadataIP, want: false},
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
		{name: "local-use NAT64", ip: "64:ff9b:1::a00:1", want: false},
		{name: "Teredo", ip: "2001::1", want: false},
		{name: "6to4", ip: "2002::1", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			require.NotNil(t, ip, "invalid test fixture %q", tc.ip)

			assert.Equal(t, tc.want, IsPubliclyRoutable(ip))
		})
	}
}

func TestIsPubliclyRoutableRejectsNilIP(t *testing.T) {
	assert.False(t, IsPubliclyRoutable(nil))
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
			resolved:     []string{publicIP},
			wantDialedTo: "8.8.8.8:443",
		},
		{
			name:        "private address is blocked",
			resolved:    []string{"10.0.0.1"},
			wantBlocked: true,
		},
		{
			name:        "metadata endpoint is blocked",
			resolved:    []string{metadataIP},
			wantBlocked: true,
		},
		{
			name:        "blocked when any answer is non-public",
			resolved:    []string{publicIP, "127.0.0.1"},
			wantBlocked: true,
		},
		{
			name:         "first public answer is used",
			resolved:     []string{"1.1.1.1", publicIP},
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

			_, err := PublicOnlyDialContext(resolve, dial)(context.Background(), "tcp", "example.com:443")

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

	_, err := PublicOnlyDialContext(resolve, dial)(context.Background(), "tcp", "example.com:443")
	require.Error(t, err)
}

// A host name whose answer changes between connections is judged on each
// answer, and the connection goes to the very address that was judged, so a
// name that first points at a public address cannot later be used to reach a
// private one.
func TestPublicOnlyDialContextChangingDNSAnswer(t *testing.T) {
	answers := []string{publicIP, metadataIP}
	var lookups int
	resolve := func(_ context.Context, _ string) ([]net.IPAddr, error) {
		ip := answers[lookups]
		lookups++
		return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil
	}

	var dialed []string
	dial := func(_ context.Context, _, addr string) (net.Conn, error) {
		dialed = append(dialed, addr)
		return nil, nil
	}

	dialContext := PublicOnlyDialContext(resolve, dial)

	_, err := dialContext(context.Background(), "tcp", "rebind.example.com:443")
	require.NoError(t, err)

	_, err = dialContext(context.Background(), "tcp", "rebind.example.com:443")
	require.ErrorIs(t, err, ErrBlockedTarget)

	assert.Equal(t, []string{"8.8.8.8:443"}, dialed)
}

// A transport may carry its own TLS dialers, which net/http uses for https
// requests in place of DialContext. A restricted transport must not keep any
// of them, or https destinations would go unchecked.
func TestRestrictTransportDropsTLSDialers(t *testing.T) {
	var requests int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Ignores the address it is given and connects to the test server
	dialTLS := func(ctx context.Context, network, _ string) (net.Conn, error) {
		dialer := &tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true}} // #nosec G402 -- test server
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}

	testCases := []struct {
		name      string
		transport func() *http.Transport
	}{
		{name: "DialTLSContext", transport: func() *http.Transport {
			return &http.Transport{DialTLSContext: dialTLS}
		}},
		{name: "DialTLS", transport: func() *http.Transport {
			return &http.Transport{DialTLS: func(network, addr string) (net.Conn, error) { //nolint:staticcheck // the deprecated field is still honored
				return dialTLS(context.Background(), network, addr)
			}}
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Positive control: the custom dialer reaches the server
			resp, err := (&http.Client{Transport: tc.transport()}).Get(server.URL)
			require.NoError(t, err)
			resp.Body.Close()

			requests = 0
			_, err = (&http.Client{Transport: RestrictTransport(tc.transport())}).Get(server.URL)
			require.ErrorIs(t, err, ErrBlockedTarget)
			assert.Zero(t, requests, "a blocked destination must not be reached")
		})
	}
}

func TestRestrictTransport(t *testing.T) {
	// httptest listens on the loopback interface, so it stands in for any
	// destination a restricted transport must refuse to reach.
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Positive control: the destination is reachable without the guard.
	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, 1, requests)

	testCases := []struct {
		name      string
		transport func() *http.Transport
	}{
		{name: "nil transport", transport: func() *http.Transport { return RestrictTransport(nil) }},
		{name: "transport with a proxy and its own dialer", transport: func() *http.Transport {
			return RestrictTransport(&http.Transport{
				Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "127.0.0.1:1"}),
				DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					// Ignores the address it is given, which must not let the
					// restricted transport reach a blocked destination.
					return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
				},
			})
		}},
		{name: "shared transport", transport: Transport},
		{name: "client transport", transport: func() *http.Transport {
			return NewHTTPClient(0).Transport.(*http.Transport)
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			requests = 0
			transport := tc.transport()

			// A proxy would be the only address the transport connects to,
			// leaving the destination unchecked.
			assert.Nil(t, transport.Proxy)

			_, err := (&http.Client{Transport: transport}).Get(server.URL)
			require.ErrorIs(t, err, ErrBlockedTarget)
			assert.Zero(t, requests, "a blocked destination must not be reached")
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
			return []net.IPAddr{{IP: net.ParseIP(publicIP)}}, nil
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

	client := &http.Client{Transport: &http.Transport{DialContext: PublicOnlyDialContext(resolve, dial)}}

	_, err := client.Get(redirector.URL)
	require.ErrorIs(t, err, ErrBlockedTarget)
	assert.Equal(t, 2, resolved, "every hop must be resolved and validated")
	assert.Zero(t, targetRequests, "the redirect target must not be reached")
}
