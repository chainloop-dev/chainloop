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

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/chainloop-dev/chainloop/app/cli/plugins"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	stringFlagType = "string"
	boolFlagType   = "bool"
	intFlagType    = "int"
)

var (
	pluginManager      *plugins.Manager
	registeredCommands map[string]string // Track which plugin registered which command
)

func init() {
	registeredCommands = make(map[string]string)
}

func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage plugins",
		Long:  "Manage Chainloop CLI plugins",
	}

	cmd.AddCommand(newPluginListCmd())
	cmd.AddCommand(newPluginDescribeCmd())
	cmd.AddCommand(newPluginDownloadCmd())

	return cmd
}

func createPluginCommand(plugin *plugins.LoadedPlugin, cmdInfo plugins.CommandInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdInfo.Name,
		Short: cmdInfo.Description,
		Long:  fmt.Sprintf("%s\n\nProvided by plugin: %s v%s", cmdInfo.Description, plugin.Metadata.Name, plugin.Metadata.Version),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Collect arguments
			arguments := make(map[string]interface{})

			// Collect flag values
			for _, flag := range cmdInfo.Flags {
				switch flag.Type {
				case stringFlagType:
					if val, err := cmd.Flags().GetString(flag.Name); err == nil {
						arguments[flag.Name] = val
					}
				case boolFlagType:
					if val, err := cmd.Flags().GetBool(flag.Name); err == nil {
						arguments[flag.Name] = val
					}
				case intFlagType:
					if val, err := cmd.Flags().GetInt(flag.Name); err == nil {
						arguments[flag.Name] = val
					}
				}
			}

			arguments["args"] = args

			// prepare Viper configuration for serialization and sending to the plugin durign execution
			viperConfig := make(map[string]interface{})
			for _, key := range viper.AllKeys() {
				viperConfig[key] = viper.Get(key)
			}

			serializedConfig, err := json.Marshal(viperConfig)
			if err != nil {
				return fmt.Errorf("error while serializing viper config: %w", err)
			}
			arguments["viper_config"] = string(serializedConfig)

			// Execute plugin command using the action pattern
			result, err := action.NewPluginExec(actionOpts, pluginManager).Run(ctx, plugin.Metadata.Name, cmdInfo.Name, arguments)
			if err != nil {
				return fmt.Errorf("failed to execute plugin command: %w", err)
			}

			// Handle result
			if result.Error != "" {
				return fmt.Errorf("the plugin command failed: %s", result.Error)
			}

			fmt.Print(result.Output)

			// Return with appropriate exit code
			if result.ExitCode != 0 {
				os.Exit(result.ExitCode)
			}

			return nil
		},
	}

	// Add flags
	for _, flag := range cmdInfo.Flags {
		switch flag.Type {
		case stringFlagType:
			defaultVal, _ := flag.Default.(string)
			cmd.Flags().String(flag.Name, defaultVal, flag.Description)
		case boolFlagType:
			defaultVal, _ := flag.Default.(bool)
			cmd.Flags().Bool(flag.Name, defaultVal, flag.Description)
		case intFlagType:
			defaultVal, _ := flag.Default.(int)
			cmd.Flags().Int(flag.Name, defaultVal, flag.Description)
		}

		if flag.Required {
			err := cmd.MarkFlagRequired(flag.Name)
			cobra.CheckErr(err)
		}
	}

	return cmd
}

func newPluginListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List installed plugins and their commands",
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := action.NewPluginList(actionOpts, pluginManager).Run(context.Background())
			if err != nil {
				return err
			}

			if flagOutputFormat == formatJSON {
				type pluginInfo struct {
					Name        string   `json:"name"`
					Version     string   `json:"version"`
					Description string   `json:"description"`
					Path        string   `json:"path"`
					Commands    []string `json:"commands"`
				}

				var items []pluginInfo
				for name, plugin := range result.Plugins {
					var commands []string
					for _, cmd := range plugin.Metadata.Commands {
						commands = append(commands, cmd.Name)
					}

					items = append(items, pluginInfo{
						Name:        name,
						Version:     plugin.Metadata.Version,
						Description: plugin.Metadata.Description,
						Path:        plugin.Path,
						Commands:    commands,
					})
				}

				return encodeJSON(items)
			}

			pluginListTableOutput(result.Plugins, result.CommandsMap)

			return nil
		},
	}
}

func newPluginDescribeCmd() *cobra.Command {
	var pluginName string

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Show detailed information about a plugin",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if pluginName == "" {
				return fmt.Errorf("plugin name is required")
			}

			result, err := action.NewPluginDescribe(actionOpts, pluginManager).Run(context.Background(), pluginName)
			if err != nil {
				return err
			}

			if flagOutputFormat == formatJSON {
				type pluginDetail struct {
					Name        string                `json:"name"`
					Version     string                `json:"version"`
					Description string                `json:"description"`
					Path        string                `json:"path"`
					Commands    []plugins.CommandInfo `json:"commands"`
				}

				detail := pluginDetail{
					Name:        result.Plugin.Metadata.Name,
					Version:     result.Plugin.Metadata.Version,
					Description: result.Plugin.Metadata.Description,
					Path:        result.Plugin.Path,
					Commands:    result.Plugin.Metadata.Commands,
				}

				return encodeJSON(detail)
			}

			pluginInfoTableOutput(result.Plugin)

			return nil
		},
	}

	cmd.Flags().StringVarP(&pluginName, "name", "", "", "Name of the plugin to describe (required)")
	cobra.CheckErr(cmd.MarkFlagRequired("name"))

	return cmd
}

func newPluginDownloadCmd() *cobra.Command {
	var url string
	var filename string

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download a plugin from a URL",
		Long:  "Download a plugin from a specified URL to the plugins directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			result, err := action.NewPluginDownload(actionOpts, pluginManager).Run(ctx, url, filename)
			if err != nil {
				return fmt.Errorf("failed to download plugin: %w", err)
			}

			fmt.Printf("Plugin downloaded successfully to: %s\n", result.FilePath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&url, "url", "u", "", "URL of the plugin to download (required)")
	cmd.Flags().StringVarP(&filename, "filename", "f", "", "Custom filename to save the plugin as (optional)")
	cobra.CheckErr(cmd.MarkFlagRequired("url"))

	return cmd
}

// loadAllPlugins loads all plugins and registers their commands to the root command
func loadAllPlugins(rootCmd *cobra.Command) error {
	ctx := context.Background()

	// Load all plugins from the plugins directory
	if err := pluginManager.LoadPlugins(ctx); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	// Get all loaded plugins
	allPlugins := pluginManager.GetAllPlugins()
	if len(allPlugins) == 0 {
		return nil
	}

	// Register commands from all plugins, checking for conflicts
	for pluginName, plugin := range allPlugins {
		for _, cmdInfo := range plugin.Metadata.Commands {
			if existingPlugin, exists := registeredCommands[cmdInfo.Name]; exists {
				return fmt.Errorf("command conflict: command '%s' is provided by both '%s' and '%s' plugins",
					cmdInfo.Name, existingPlugin, pluginName)
			}

			// Register the command
			pluginCmd := createPluginCommand(plugin, cmdInfo)
			rootCmd.AddCommand(pluginCmd)
			registeredCommands[cmdInfo.Name] = pluginName
		}
	}

	return nil
}

// cleanupPlugins should be called during application shutdown
func cleanupPlugins() {
	if pluginManager != nil {
		pluginManager.Shutdown()
	}
}

// Table output functions
func pluginListTableOutput(plugins map[string]*plugins.LoadedPlugin, commandsMap map[string]string) {
	if len(plugins) == 0 {
		fmt.Println("No plugins installed")
		return
	}

	t := newTableWriter()
	t.AppendHeader(table.Row{"Name", "Version", "Description", "Commands"})

	for name, plugin := range plugins {
		commandStr := fmt.Sprintf("%d command(s)", len(plugin.Metadata.Commands))
		if len(plugin.Metadata.Commands) == 0 {
			commandStr = "no commands"
		}

		t.AppendRow(table.Row{name, plugin.Metadata.Version, plugin.Metadata.Description, commandStr})
		t.AppendSeparator()
	}

	t.Render()

	t = newTableWriter()
	t.AppendHeader(table.Row{"Plugin", "Command"})
	for cmd, plugin := range commandsMap {
		t.AppendRow(table.Row{plugin, cmd})
		t.AppendSeparator()
	}
	t.Render()
}

func pluginInfoTableOutput(plugin *plugins.LoadedPlugin) {
	t := newTableWriter()

	t.AppendHeader(table.Row{"Name", "Version", "Description", "Commands"})
	t.AppendRow(table.Row{plugin.Metadata.Name, plugin.Metadata.Version, plugin.Metadata.Description, fmt.Sprintf("%d command(s)", len(plugin.Metadata.Commands))})

	t.Render()

	pluginInfoCommandsTableOutput(plugin)
	pluginInfoFlagsTableOutput(plugin)
}

func pluginInfoCommandsTableOutput(plugin *plugins.LoadedPlugin) {
	t := newTableWriter()

	t.AppendHeader(table.Row{"Plugin", "Command", "Description", "Usage"})
	for _, cmd := range plugin.Metadata.Commands {
		t.AppendRow(table.Row{plugin.Metadata.Name, cmd.Name, cmd.Description, cmd.Usage})
		t.AppendSeparator()
	}

	t.Render()
}

func pluginInfoFlagsTableOutput(plugin *plugins.LoadedPlugin) {
	if len(plugin.Metadata.Commands) == 0 {
		return
	}

	flagsPresent := false
	for _, cmd := range plugin.Metadata.Commands {
		if len(cmd.Flags) > 0 {
			flagsPresent = true
		}
	}

	if !flagsPresent {
		return
	}

	t := newTableWriter()

	t.AppendHeader(table.Row{"Plugin", "Command", "Flag", "Description", "Type", "Default", "Required"})
	for _, cmd := range plugin.Metadata.Commands {
		for _, flag := range cmd.Flags {
			t.AppendRow(table.Row{plugin.Metadata.Name, cmd.Name, flag.Name, flag.Description, flag.Type, flag.Default, flag.Required})
			t.AppendSeparator()
		}
	}

	t.Render()
}
