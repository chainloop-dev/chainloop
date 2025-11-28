//
// Copyright 2025 The Chainloop Authors.
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

package builtins

import (
	"context"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"google.golang.org/grpc"
)

// DiscoverService wraps the gRPC discover functionality to be shared across engines
type DiscoverService struct {
	conn *grpc.ClientConn
}

// NewDiscoverService creates a new discover service
func NewDiscoverService(conn *grpc.ClientConn) *DiscoverService {
	return &DiscoverService{conn: conn}
}

// Discover calls the DiscoverPrivate gRPC endpoint to get artifact graph data
func (s *DiscoverService) Discover(ctx context.Context, digest, kind string) (*v1.ReferrerServiceDiscoverPrivateResponse, error) {
	if s.conn == nil {
		return nil, nil
	}

	client := v1.NewReferrerServiceClient(s.conn)
	return client.DiscoverPrivate(ctx, &v1.ReferrerServiceDiscoverPrivateRequest{
		Digest: digest,
		Kind:   kind,
	})
}
