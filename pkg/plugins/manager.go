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
	"path/filepath"
	"runtime"

	"github.com/hashicorp/go-plugin"
	"github.com/rs/zerolog"
)

// Manager handles loading and managing plugins.
type Manager struct {
	plugins       map[string]*LoadedPlugin
	pluginClients map[string]*plugin.Client
	logger        zerolog.Logger
}

// LoadedPlugin represents a loaded plugin with its metadata.
type LoadedPlugin struct {
	Path     string
	Plugin   Plugin
	Metadata PluginMetadata
}

// NewManager creates a new plugin manager.
func NewManager(logger zerolog.Logger) *Manager {
	return &Manager{
		plugins:       make(map[string]*LoadedPlugin),
		pluginClients: make(map[string]*plugin.Client),
		logger:        logger,
	}
}

// LoadPlugins loads all plugins from the plugins directory.
func (m *Manager) LoadPlugins(ctx context.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	pluginsDir := filepath.Join(homeDir, ".config", "chainloop", "plugins") // TODO: make this configurable

	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(pluginsDir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			m.logger.Err(err).Str("pluginPath", pluginPath).Msg("failed to get info for plugin")
			continue
		}

		// On Windows, check for .exe extension
		if runtime.GOOS == "windows" && filepath.Ext(pluginPath) != ".exe" {
			continue
		}

		// On Unix, check if executable
		if runtime.GOOS != "windows" && info.Mode()&0111 == 0 {
			continue
		}

		// Load the plugin
		if err := m.loadPlugin(ctx, pluginPath); err != nil {
			m.logger.Err(err).Str("pluginPath", pluginPath).Msg("failed to load plugin")
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

	m.logger.Debug().Str("pluginName", metadata.Name).Str("pluginVersion", metadata.Version).Msg("loaded plugin")
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
	for name, client := range m.pluginClients {
		m.logger.Debug().Str("pluginName", name).Msg("shutting down plugin")
		client.Kill()
	}
}
