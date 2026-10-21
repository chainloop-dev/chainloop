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

	"github.com/stretchr/testify/require"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
)

// TestReflectionGatedByEnv verifies that gRPC reflection is served only in
// development (Version == "dev") and disabled in production, on the shared
// control-plane endpoint. No product client depends on reflection; it is a
// local-development convenience.
func TestReflectionGatedByEnv(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	cases := []struct {
		name          string
		version       string
		wantReachable bool
	}{
		{"dev keeps reflection for local grpcurl", "dev", true},
		{"production disables reflection", "v1.2.3", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			Version = tc.version
			addr, _, _ := startSinglePortApp(t, nil)
			require.Equal(t, tc.wantReachable, cpReflectionReachable(t, addr),
				"Version=%q: reflection reachable should be %v", tc.version, tc.wantReachable)
		})
	}
}

// cpReflectionReachable reports whether the shared endpoint answers a gRPC
// reflection ListServices request with a non-empty service list.
func cpReflectionReachable(t *testing.T, addr string) bool {
	t.Helper()
	conn, err := grpcLib.NewClient(addr, grpcLib.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := reflectionpb.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	require.NoError(t, err)
	if err := stream.Send(&reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{},
	}); err != nil {
		return false // stream rejected -> reflection not served
	}
	resp, err := stream.Recv()
	if err != nil {
		return false // Unimplemented / unknown service -> reflection disabled
	}
	return len(resp.GetListServicesResponse().GetService()) > 0
}
