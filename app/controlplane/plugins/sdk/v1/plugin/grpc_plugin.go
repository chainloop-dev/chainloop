// Copyright 2023 The Chainloop Authors.
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

package plugin

import (
	"context"

	plugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1/plugin/api"
)

var _ plugin.Plugin = (*GRPCFanOutPlugin)(nil)
var _ plugin.GRPCPlugin = (*GRPCFanOutPlugin)(nil)

// GRPCFanOutPlugin is the plugin.Plugin implementation that only supports GRPC transport
type GRPCFanOutPlugin struct {
	impl sdk.FanOut
	// Embedding this will disable the netRPC protocol
	plugin.NetRPCUnsupportedPlugin
}

func (b GRPCFanOutPlugin) GRPCServer(_ *plugin.GRPCBroker, s *grpc.Server) error {
	api.RegisterFanoutServiceServer(s, &fanOutGRPCServer{
		impl: b.impl,
	})

	return nil
}

func (b *GRPCFanOutPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &fanOutGRPCClient{client: api.NewFanoutServiceClient(c)}, nil
}
