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
	"fmt"
	"math"
	"os"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
	plugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

type ServeOpts struct {
	Factory sdk.FanOutFactory
}

// Serve is a helper function used to serve a backend plugin. This
// should be ran on the plugin's main process.
func Serve(opts *ServeOpts) error {
	l := log.NewStdLogger(os.Stderr)

	impl, err := opts.Factory(l)
	if err != nil {
		return fmt.Errorf("failed to initialize plugin implementation: %w", err)
	}

	// pluginMap is the map of plugins we can dispense.
	pluginSet := plugin.PluginSet{
		PluginName: &GRPCFanOutPlugin{
			impl: impl,
		},
	}

	serveOpts := &plugin.ServeConfig{
		HandshakeConfig: HandshakeConfig,
		Plugins:         pluginSet,

		GRPCServer: func(opts []grpc.ServerOption) *grpc.Server {
			opts = append(opts, grpc.MaxRecvMsgSize(math.MaxInt32))
			opts = append(opts, grpc.MaxSendMsgSize(math.MaxInt32))
			return plugin.DefaultGRPCServer(opts)
		},
	}

	plugin.Serve(serveOpts)

	return nil
}

// Currently we only support one plugin type
const PluginName = "fanOut"

var HandshakeConfig = plugin.HandshakeConfig{
	MagicCookieKey:   "CHAINLOOP_PLUGIN",
	MagicCookieValue: "e575e823-335c-4e3b-8bfd-acceef0ae074",
}
