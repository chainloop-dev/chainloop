// Copyright 2025 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugins

import (
	"context"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
)

// ChainloopCliPlugin is the implementation of plugin.Plugin.
type ChainloopCliPlugin struct {
	Impl Plugin
}

func (p *ChainloopCliPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &RPCServer{Impl: p.Impl}, nil
}

func (ChainloopCliPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &RPCClient{client: c}, nil
}

// RPCClient is an implementation of Plugin that talks over RPC.
type RPCClient struct {
	client *rpc.Client
}

func (m *RPCClient) Exec(ctx context.Context, command string, arguments map[string]any) (ExecResult, error) {
	var resp ExecResponse
	err := m.client.Call("Plugin.Exec", map[string]any{
		"command":   command,
		"arguments": arguments,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (m *RPCClient) GetMetadata(ctx context.Context) (PluginMetadata, error) {
	var resp PluginMetadata
	err := m.client.Call("Plugin.GetMetadata", new(any), &resp)
	return resp, err
}

// RPCServer is the RPC server that RPCClient talks to, conforming to the requirements of net/rpc.
type RPCServer struct {
	Impl Plugin
}

func (m *RPCServer) Exec(args map[string]any, resp *ExecResponse) error {
	ctx := context.Background()
	command := args["command"].(string)
	arguments := args["arguments"].(map[string]any)

	result, err := m.Impl.Exec(ctx, command, arguments)
	if err != nil {
		return err
	}

	*resp = ExecResponse{
		Output:   result.GetOutput(),
		Error:    result.GetError(),
		ExitCode: result.GetExitCode(),
		Data:     result.GetData(),
	}
	return nil
}

func (m *RPCServer) GetMetadata(args any, resp *PluginMetadata) error {
	metadata, err := m.Impl.GetMetadata(context.Background())
	if err != nil {
		return err
	}
	*resp = metadata
	return nil
}

// ExecResponse is a concrete implementation of ExecResult for RPC.
type ExecResponse struct {
	Output   string
	Error    string
	ExitCode int
	Data     map[string]any
}

func (r *ExecResponse) GetOutput() string {
	return r.Output
}

func (r *ExecResponse) GetError() string {
	return r.Error
}

func (r *ExecResponse) GetExitCode() int {
	return r.ExitCode
}

func (r *ExecResponse) GetData() map[string]any {
	return r.Data
}
