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
	"sync/atomic"
	"testing"
	"time"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// The control plane serves only unary RPCs. Streaming methods it does not
// implement are rejected with Unimplemented before reaching a handler, while
// gRPC reflection (a development-only convenience) remains available.
func TestServerServesUnaryOnlyExceptReflection(t *testing.T) {
	off := false
	opts := testServerOpts(t, &conf.Server{
		Grpc:               &conf.Server_GRPC{Addr: testAnyAddr},
		Http:               &conf.Server_HTTP{Addr: testAnyAddr},
		LegacyGrpcListener: &off,
	})
	gs, err := NewGRPCServers(opts)
	require.NoError(t, err)

	// Register a streaming service on the real server so the test exercises the
	// stream interceptor on a registered method (rather than gRPC's handling of an
	// unknown method). The handler records whether it is ever reached.
	var handlerCalled atomic.Bool
	const probeMethod = "/chainloop.test.v1.StreamProbe/Open"
	gs.Handler.RegisterService(&grpcLib.ServiceDesc{
		ServiceName: "chainloop.test.v1.StreamProbe",
		HandlerType: (*interface{})(nil),
		Streams: []grpcLib.StreamDesc{
			{
				StreamName: "Open",
				Handler: func(_ interface{}, stream grpcLib.ServerStream) error {
					handlerCalled.Store(true)
					return stream.RecvMsg(new(emptypb.Empty))
				},
				ServerStreams: true,
				ClientStreams: true,
			},
		},
	}, nil)

	api, err := NewHTTPServer(opts, gs)
	require.NoError(t, err)
	endpoint, err := api.Endpoint()
	require.NoError(t, err)
	go func() { _ = api.Start(context.Background()) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = api.Stop(ctx)
	})

	conn, err := grpcLib.NewClient(endpoint.Host, grpcLib.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	t.Run("registered streaming method is rejected before its handler runs", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		desc := &grpcLib.StreamDesc{StreamName: "Open", ServerStreams: true, ClientStreams: true}
		stream, err := conn.NewStream(ctx, desc, probeMethod)
		if err == nil {
			_ = stream.SendMsg(&emptypb.Empty{})
			err = stream.RecvMsg(&emptypb.Empty{})
		}
		assert.Equal(t, codes.Unimplemented, status.Code(err))
		assert.False(t, handlerCalled.Load(), "the streaming handler must not be invoked")
	})

	t.Run("reflection stream is still served in development", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stream, err := reflectionpb.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
		require.NoError(t, err)
		require.NoError(t, stream.Send(&reflectionpb.ServerReflectionRequest{
			MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{ListServices: ""},
		}))
		resp, err := stream.Recv()
		require.NoError(t, err)
		assert.NotEmpty(t, resp.GetListServicesResponse().GetService())
	})
}
