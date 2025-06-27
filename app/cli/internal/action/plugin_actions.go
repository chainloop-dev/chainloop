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
	"os"
	"path/filepath"

	"github.com/chainloop-dev/chainloop/app/cli/common"
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

// PluginInstall handles downloading a plugin
type PluginInstall struct {
	cfg     *ActionsOpts
	manager *plugins.Manager
}

// PluginInstallOptions contains all options for installing a plugin
type PluginInstallOptions struct {
	File     string
	Filename string
	Location string
}

// PluginInstallResult represents the result of installing a plugin
type PluginInstallResult struct {
	FilePath string
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
func (action *PluginExec) Run(ctx context.Context, pluginName string, commandName string, config plugins.PluginExecConfig) (*PluginExecResult, error) {
	action.cfg.Logger.Debug().Str("pluginName", pluginName).Str("command", commandName).Msg("Executing plugin command")
	plugin, ok := action.manager.GetPlugin(pluginName)
	if !ok {
		return nil, fmt.Errorf("plugin '%s' not found", pluginName)
	}

	result, err := plugin.Plugin.Exec(ctx, config)
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

// NewPluginInstall creates a new PluginInstall action
func NewPluginInstall(cfg *ActionsOpts, manager *plugins.Manager) *PluginInstall {
	return &PluginInstall{cfg: cfg, manager: manager}
}

// Run executes the PluginInstall action
func (action *PluginInstall) Run(ctx context.Context, opts *PluginInstallOptions) (*PluginInstallResult, error) {
	action.cfg.Logger.Debug().Str("file", opts.File).Msg("Downloading plugin")

	// create plugins directory if it doesn't exist
	pluginsDir := common.GetPluginsDir()
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		action.cfg.Logger.Error().Err(err).Str("directory", pluginsDir).Msg("Failed to create plugins directory")
		return nil, fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// determine filename
	filename := filepath.Base(opts.File)
	if opts.Filename != "" {
		filename = opts.Filename
		action.cfg.Logger.Debug().Str("customFilename", opts.Filename).Msg("Using custom filename")
	} else if filename == "." || filename == "/" {
		action.cfg.Logger.Error().Str("file", opts.File).Msg("Invalid URL or object key, could not determine filename")
		return nil, fmt.Errorf("invalid URL or object key, could not determine filename")
	}

	filePath := filepath.Join(pluginsDir, filename)

	// create a temporary file for downloading
	tempFilePath := filePath + ".tmp"
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		action.cfg.Logger.Error().Err(err).Str("path", tempFilePath).Msg("Failed to create temporary file")
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer tempFile.Close()

	rawFileContents, err := LoadFileOrURL(opts.File)
	if err != nil {
		return nil, fmt.Errorf("failed to load file from a given URL: %w", err)
	}

	// write to the temporary file
	_, err = tempFile.Write(rawFileContents)
	if err != nil {
		return nil, fmt.Errorf("failed to write file to temporary file: %w", err)
	}

	// close the file before renaming
	tempFile.Close()

	// rename the temporary file to the final file
	if err := os.Rename(tempFilePath, filePath); err != nil {
		os.Remove(tempFilePath)
		action.cfg.Logger.Error().Err(err).Str("tempPath", tempFilePath).Str("finalPath", filePath).Msg("Failed to rename temporary file")
		return nil, fmt.Errorf("failed to rename temporary file: %w", err)
	}

	// Set executable permissions for the plugin file
	// This is needed, so we can run the plugin and serve the gRPC
	if err := os.Chmod(filePath, 0755); err != nil {
		action.cfg.Logger.Error().Err(err).Str("path", filePath).Msg("Failed to set executable permissions")
		return nil, fmt.Errorf("failed to set executable permissions: %w", err)
	}

	action.cfg.Logger.Debug().Str("file", opts.File).Str("path", filePath).Msg("Plugin downloaded successfully")
	return &PluginInstallResult{
		FilePath: filePath,
	}, nil
}
