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

package action

import (
	"context"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/plugins"
)

// PluginList handles listing installed plugins
type PluginList struct {
	cfg     *ActionsOpts
	manager *plugins.Manager
}

// PluginInfo handles showing detailed information about a specific plugin
type PluginDescribe struct {
	cfg     *ActionsOpts
	manager *plugins.Manager
}

// PluginExec handles executing a command provided by a plugin
type PluginExec struct {
	cfg     *ActionsOpts
	manager *plugins.Manager
}

// PluginListResult represents the result of listing plugins
type PluginListResult struct {
	Plugins     map[string]*plugins.LoadedPlugin
	CommandsMap map[string]string // Maps command names to plugin names
}

// PluginDescribeResult represents the result of getting plugin info
type PluginDescribeResult struct {
	Plugin *plugins.LoadedPlugin
}

// PluginExecResult represents the result of executing a plugin command
type PluginExecResult struct {
	Output   string
	Error    string
	ExitCode int
	Data     map[string]any
}

// NewPluginList creates a new PluginList action
func NewPluginList(cfg *ActionsOpts, manager *plugins.Manager) *PluginList {
	return &PluginList{cfg: cfg, manager: manager}
}

// Run executes the PluginList action
func (action *PluginList) Run(_ context.Context) (*PluginListResult, error) {
	action.cfg.Logger.Debug().Msg("Listing all plugins")
	plugins := action.manager.GetAllPlugins()

	// Create a map of command names to plugin names
	commandsMap := make(map[string]string)
	for pluginName, plugin := range plugins {
		for _, cmd := range plugin.Metadata.Commands {
			commandsMap[cmd.Name] = pluginName
		}
	}

	action.cfg.Logger.Debug().Int("pluginCount", len(plugins)).Int("commandCount", len(commandsMap)).Msg("Found plugins and commands")
	return &PluginListResult{
		Plugins:     plugins,
		CommandsMap: commandsMap,
	}, nil
}

// NewPluginDescribe creates a new NewPluginDescribe action
func NewPluginDescribe(cfg *ActionsOpts, manager *plugins.Manager) *PluginDescribe {
	return &PluginDescribe{cfg: cfg, manager: manager}
}

// Run executes the NewPluginDescribe action
func (action *PluginDescribe) Run(_ context.Context, pluginName string) (*PluginDescribeResult, error) {
	action.cfg.Logger.Debug().Str("pluginName", pluginName).Msg("Getting plugin info")
	plugin, ok := action.manager.GetPlugin(pluginName)
	if !ok {
		return nil, fmt.Errorf("plugin '%s' not found", pluginName)
	}

	action.cfg.Logger.Debug().Str("pluginName", pluginName).Str("version", plugin.Metadata.Version).Int("commandCount", len(plugin.Metadata.Commands)).Msg("Found plugin")
	return &PluginDescribeResult{
		Plugin: plugin,
	}, nil
}

// NewPluginExec creates a new PluginExec action
func NewPluginExec(cfg *ActionsOpts, manager *plugins.Manager) *PluginExec {
	return &PluginExec{cfg: cfg, manager: manager}
}

// Run executes the PluginExec action
func (action *PluginExec) Run(ctx context.Context, pluginName string, commandName string, arguments map[string]interface{}) (*PluginExecResult, error) {
	action.cfg.Logger.Debug().Str("pluginName", pluginName).Str("command", commandName).Msg("Executing plugin command")
	plugin, ok := action.manager.GetPlugin(pluginName)
	if !ok {
		return nil, fmt.Errorf("plugin '%s' not found", pluginName)
	}

	result, err := plugin.Plugin.Exec(ctx, commandName, arguments)
	if err != nil {
		action.cfg.Logger.Error().Err(err).Str("pluginName", pluginName).Str("command", commandName).Msg("Plugin execution failed")
		return nil, fmt.Errorf("plugin execution failed: %w", err)
	}

	if result.GetError() != "" {
		action.cfg.Logger.Error().Str("pluginName", pluginName).Str("command", commandName).Str("error", result.GetError()).Msg("Plugin returned error")
	}

	action.cfg.Logger.Debug().Str("pluginName", pluginName).Str("command", commandName).Int("exitCode", result.GetExitCode()).Msg("Plugin command executed")
	return &PluginExecResult{
		Output:   result.GetOutput(),
		Error:    result.GetError(),
		ExitCode: result.GetExitCode(),
		Data:     result.GetData(),
	}, nil
}
