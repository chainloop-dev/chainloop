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

package plugins

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/chainloop-dev/chainloop/app/cli/common"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

// Manager handles loading and managing plugins.
type Manager struct {
	plugins       map[string]*LoadedPlugin
	pluginClients map[string]*plugin.Client
}

// LoadedPlugin represents a loaded plugin with its metadata.
type LoadedPlugin struct {
	Path     string
	Plugin   Plugin
	Metadata PluginMetadata
}

// NewManager creates a new plugin manager.
func NewManager() *Manager {
	return &Manager{
		plugins:       make(map[string]*LoadedPlugin),
		pluginClients: make(map[string]*plugin.Client),
	}
}

// LoadPlugins loads all plugins from the plugins directory.
func (m *Manager) LoadPlugins(ctx context.Context) error {
	pluginsDir := common.GetPluginsDir()

	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Use appropriate glob pattern based on OS
	glob := "*"
	if common.IsWindows() {
		glob = "*.exe"
	}

	plugins, err := plugin.Discover(glob, pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to discover plugins: %w", err)
	}

	for _, plugin := range plugins {
		// Load the plugin - if there is an error just skip it - we can think of a better strategy later
		if err := m.loadPlugin(ctx, plugin); err != nil {
			fmt.Printf("failed to load plugin: %s\n", err)
			continue
		}
	}

	return nil
}

// loadPlugin loads a single plugin.
func (m *Manager) loadPlugin(ctx context.Context, path string) error {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins:         PluginMap,
		Cmd:             exec.Command(path),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolNetRPC,
		},
		// By default the go-plugin logger is set to TRACE level, which is very verbose.
		// We can't set it to the level set by the command at this point, because we need to
		// load the commands first before running the cobra command where the debug flag is set
		// We set it to WARN level, so we don't get too much noise from the plugins.
		Logger: hclog.New(&hclog.LoggerOptions{
			Output: hclog.DefaultOutput,
			Level:  hclog.Trace,
			Name:   "plugin",
		}),
	})

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return fmt.Errorf("failed to create RPC client: %w", err)
	}

	// Request the plugin
	raw, err := rpcClient.Dispense("chainloop")
	if err != nil {
		client.Kill()
		return fmt.Errorf("failed to dispense plugin: %w", err)
	}

	// Cast to our interface
	chainloopPlugin, ok := raw.(Plugin)
	if !ok {
		client.Kill()
		return fmt.Errorf("plugin does not implement Plugin interface")
	}

	// Get plugin metadata
	metadata, err := chainloopPlugin.GetMetadata(ctx)
	if err != nil {
		client.Kill()
		return fmt.Errorf("failed to get plugin metadata: %w", err)
	}

	// Store the plugin
	m.plugins[metadata.Name] = &LoadedPlugin{
		Path:     path,
		Plugin:   chainloopPlugin,
		Metadata: metadata,
	}
	m.pluginClients[metadata.Name] = client

	return nil
}

// GetPlugin returns a loaded plugin by name.
func (m *Manager) GetPlugin(name string) (*LoadedPlugin, bool) {
	plugin, ok := m.plugins[name]
	return plugin, ok
}

// GetAllPlugins returns all loaded plugins.
func (m *Manager) GetAllPlugins() map[string]*LoadedPlugin {
	return m.plugins
}

// Shutdown closes all plugin connections.
func (m *Manager) Shutdown() {
	for _, client := range m.pluginClients {
		client.Kill()
	}
}
