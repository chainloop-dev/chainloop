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
	"errors"
	"io"
	"net"
	"testing"
	"time"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/soheilhy/cmux"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func TestCASClassificationTimeout(t *testing.T) {
	for _, prefix := range []string{"", "PRI ", "GET / HTTP/1.1\r\n"} {
		t.Run(prefix, func(t *testing.T) {
			opts := casTestOpts(t)
			opts.ServerConfig.LegacyGrpcListener = new(false)
			servers, err := NewGRPCServers(opts)
			require.NoError(t, err)
			api, err := NewHTTPServer(opts, servers)
			require.NoError(t, err)
			// Exercise the lifecycle timer, including a client that never reads
			// server SETTINGS, independently of cmux's read deadline.
			api.lis.(*classificationListener).timeout = 50 * time.Millisecond
			api.httpSrv.ReadHeaderTimeout = 50 * time.Millisecond
			endpoint, err := api.Endpoint()
			require.NoError(t, err)
			startCASTestTransport(t, api)
			conn, err := net.Dial("tcp", endpoint.Host)
			require.NoError(t, err)
			defer conn.Close()
			_, err = io.WriteString(conn, prefix)
			require.NoError(t, err)
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
			_, err = io.Copy(io.Discard, conn)
			require.NoError(t, err, "classification must close before the client deadline")
		})
	}
}

func TestCASClassificationBudget(t *testing.T) {
	opts := casTestOpts(t)
	opts.ServerConfig.LegacyGrpcListener = new(false)
	servers, err := NewGRPCServers(opts)
	require.NoError(t, err)
	api, err := NewHTTPServer(opts, servers)
	require.NoError(t, err)
	endpoint, err := api.Endpoint()
	require.NoError(t, err)
	startCASTestTransport(t, api)
	conn, err := net.Dial("tcp", endpoint.Host)
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.SetDeadline(time.Now().Add(time.Second)))
	// A peer may send valid HTTP/2 frames forever without the HEADERS cmux
	// needs. The wire-byte budget must stop it before the time limit.
	_, err = io.WriteString(conn, http2.ClientPreface)
	require.NoError(t, err)
	framer := http2.NewFramer(conn, conn)
	require.NoError(t, framer.WriteSettings())
	frame, err := framer.ReadFrame()
	require.NoError(t, err)
	require.IsType(t, &http2.SettingsFrame{}, frame)
	// Padded DATA are irrelevant to the header matcher but consume its budget.
	for range maxClassificationBytes / (16 << 10) {
		if err = framer.WriteData(1, false, make([]byte, 16<<10)); err != nil {
			break
		}
	}
	_, err = io.Copy(io.Discard, conn)
	var netErr net.Error
	if errors.As(err, &netErr) {
		require.False(t, netErr.Timeout(), "classification byte limit was not enforced")
	}
}

func TestCASClassificationConcurrencyLimit(t *testing.T) {
	root, err := net.Listen("tcp", casAnyAddr)
	require.NoError(t, err)
	listener := &classificationListener{Listener: root, pending: make(map[*classificationConn]*time.Timer), timeout: time.Minute}
	defer listener.Close()
	mux := cmux.New(listener)
	mux.SetReadTimeout(time.Minute)
	mux.Match(cmux.HTTP1Fast())
	done := make(chan error, 1)
	go func() { done <- mux.Serve() }()
	for range maxClassifyingConnections {
		conn, err := net.Dial("tcp", root.Addr().String())
		require.NoError(t, err)
		defer conn.Close()
	}
	require.Eventually(t, func() bool {
		listener.mu.Lock()
		defer listener.mu.Unlock()
		return len(listener.pending) == maxClassifyingConnections
	}, time.Second, time.Millisecond)
	rejected, err := net.Dial("tcp", root.Addr().String())
	require.NoError(t, err)
	defer rejected.Close()
	require.NoError(t, rejected.SetReadDeadline(time.Now().Add(time.Second)))
	var data [1]byte
	_, err = rejected.Read(data[:])
	require.ErrorIs(t, err, io.EOF)
	require.NoError(t, listener.Close())
	mux.Close()
	select {
	case err := <-done:
		require.ErrorIs(t, err, net.ErrClosed)
	case <-time.After(time.Second):
		require.FailNow(t, "classifier workers did not exit")
	}
}

func TestCASClassificationDeadlineCleared(t *testing.T) {
	opts := casTestOpts(t)
	opts.ServerConfig.LegacyGrpcListener = new(false)
	servers, err := NewGRPCServers(opts)
	require.NoError(t, err)
	api, err := NewHTTPServer(opts, servers)
	require.NoError(t, err)
	api.lis.(*classificationListener).timeout = 250 * time.Millisecond
	api.mux.SetReadTimeout(250 * time.Millisecond)
	endpoint, err := api.Endpoint()
	require.NoError(t, err)
	startCASTestTransport(t, api)
	conn := casTestConn(t, endpoint.Host, false)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// A gRPC connection must remain usable after its classification deadline.
	client := v1.NewStatusServiceClient(conn)
	_, err = client.Statusz(ctx, &v1.StatuszRequest{})
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)
	_, err = client.Statusz(ctx, &v1.StatuszRequest{})
	require.NoError(t, err)
}
