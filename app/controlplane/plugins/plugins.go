//
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

package plugins

import (
	"fmt"
	"os/exec"
	"sort"

	dependencytrack "github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/dependency-track/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/discord-webhook/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/guac/v1"
	ociregistry "github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/oci-registry/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/slack-webhook/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/smtp/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	plugin_sdk "github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1/plugin"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/hashicorp/go-plugin"
)

type pluginsLoader interface {
	load() (sdk.AvailablePlugins, error)
}

// directoryLoader loads plugins from a directory
type directoryLoader struct {
	pluginsDir  string
	logger      *log.Helper
	initializer PluginInitializer
}

// memoryLoader initializes plugins in memory from an array of factories
type memoryLoader struct {
	plugins []sdk.FanOutFactory
	logger  log.Logger
}

type PluginInitializer interface {
	Init(path string) (*sdk.FanOutP, error)
}

// Init a go-plugin plugin
type goPluginInitializer struct{}

// Load the available third party integrations, these integrations can come in the form of
// a) Plugins implemented with go-plugin, compiled as a separate binary and placed in pluginsDir
// b) Built-in plugins implemented as a go modules and loaded in memory
// Important: Plugins have precedence over built-in plugins
func Load(pluginsDir string, l log.Logger) (plugins sdk.AvailablePlugins, err error) {
	// Array of built-in plugins to enable which are loaded in host memory dynamically
	toEnableBuiltIn := []sdk.FanOutFactory{
		dependencytrack.New,
		smtp.New,
		ociregistry.New,
		discord.New,
		guac.New,
		slack.New,
	}

	// Load plugins in memory from the array above
	memLoader := &memoryLoader{plugins: toEnableBuiltIn, logger: l}

	logger := servicelogger.ScopedHelper(l, "plugins")
	// Load plugins from a directory
	dirLoader := &directoryLoader{
		pluginsDir:  pluginsDir,
		initializer: &goPluginInitializer{},
		logger:      logger,
	}

	return doLoad(memLoader, dirLoader, logger)
}

func doLoad(memoryLoader pluginsLoader, dirLoader pluginsLoader, logger *log.Helper) (plugins sdk.AvailablePlugins, err error) {
	defer func() {
		if err != nil {
			// If there is an error, we need to clean up the plugins that were loaded
			plugins.Cleanup()
		}
	}()

	// Get actual plugins running processes
	plugins, err = dirLoader.load()
	if err != nil {
		return plugins, fmt.Errorf("failed to load plugins: %w", err)
	}

	fromMemoryPlugins, err := memoryLoader.load()
	if err != nil {
		return plugins, fmt.Errorf("failed to load plugins: %w", err)
	}

	// load built-in plugins but skip them if they are loaded already as actual plugins
	// It will enable us to rollout plugins iteratively
BUILT_IN_LOOP:
	for _, builtInPlugin := range fromMemoryPlugins {
		for _, p := range plugins {
			if p.Describe().ID == builtInPlugin.Describe().ID {
				logger.Infow("msg", "plugin already loaded", "type", "built-in", "plugin", p.String())
				continue BUILT_IN_LOOP
			}
		}

		logger.Infow("msg", "loaded", "type", "built-in", "plugin", builtInPlugin.String())
		plugins = append(plugins, &sdk.FanOutP{FanOut: builtInPlugin, DisposeFunc: func() {}})
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Describe().ID < plugins[j].Describe().ID
	})

	return plugins, nil
}

func (l *memoryLoader) load() (sdk.AvailablePlugins, error) {
	res := make(sdk.AvailablePlugins, 0, len(l.plugins))
	for _, f := range l.plugins {
		p, err := f(l.logger)
		if err != nil {
			return nil, fmt.Errorf("failed to load built-in plugin: %w", err)
		}

		res = append(res, &sdk.FanOutP{FanOut: p, DisposeFunc: func() {}})
	}

	return res, nil
}

func (l *directoryLoader) load() (sdk.AvailablePlugins, error) {
	var plugins = make(sdk.AvailablePlugins, 0)
	if l.pluginsDir == "" {
		return plugins, nil
	}

	const pluginPrefix = "chainloop-plugin-"
	var pluginBlob = fmt.Sprintf("%s*", pluginPrefix)

	l.logger.Infow("msg", "loading plugins", "dir", l.pluginsDir, "pattern", pluginBlob)
	files, err := plugin.Discover(pluginBlob, l.pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to discover plugins: %w", err)
	}

	var pluginsMap = make(map[string]*sdk.FanOutP)
	for _, f := range files {
		d, err := l.initializer.Init(f)
		if err != nil {
			return nil, fmt.Errorf("failed to load plugin: %w", err)
		}

		if _, ok := pluginsMap[d.Describe().ID]; ok {
			l.logger.Infow("msg", "duplicate plugin, skipping", "plugin", d.Describe().ID)
			d.DisposeFunc()
			continue
		}

		pluginsMap[d.Describe().ID] = d
	}

	for _, p := range pluginsMap {
		l.logger.Infow("msg", "loaded", "type", "plugin", "plugin", p.String())
		plugins = append(plugins, p)
	}

	return plugins, nil
}

func (i *goPluginInitializer) Init(path string) (*sdk.FanOutP, error) {
	pluginSet := plugin.PluginSet{
		plugin_sdk.PluginName: &plugin_sdk.GRPCFanOutPlugin{},
	}

	// Plugin load test
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  plugin_sdk.HandshakeConfig,
		Plugins:          pluginSet,
		Cmd:              exec.Command(path),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		AutoMTLS:         true,
	})

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		return nil, err
	}

	// Request the plugin
	raw, err := rpcClient.Dispense(plugin_sdk.PluginName)
	if err != nil {
		return nil, err
	}

	fanOut, ok := raw.(sdk.FanOut)
	if !ok {
		return nil, fmt.Errorf("plugin %q does not implement the FanOut interface", path)
	}

	return &sdk.FanOutP{FanOut: fanOut, DisposeFunc: client.Kill}, nil
}
