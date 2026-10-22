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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
)

// The control plane serves only unary RPCs. Streaming methods it does not
// implement are rejected with Unimplemented, while gRPC reflection (a
// development-only convenience) remains available.
func TestServerServesUnaryOnlyExceptReflection(t *testing.T) {
	addr, _, _ := startSinglePortApp(t, nil)
	conn, err := grpcLib.NewClient(addr, grpcLib.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	t.Run("unsupported streaming method returns Unimplemented", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stream, err := grpc_health_v1.NewHealthClient(conn).Watch(ctx, &grpc_health_v1.HealthCheckRequest{})
		require.NoError(t, err)
		_, err = stream.Recv()
		assert.Equal(t, codes.Unimplemented, status.Code(err))
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
