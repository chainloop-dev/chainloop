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
	"fmt"
	"os"
	"strconv"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/plugins"
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
		Short: "Manage plugins (preview)",
		Long:  "Manage Chainloop CLI plugins (preview)",
	}

	cmd.AddCommand(newPluginListCmd())
	cmd.AddCommand(newPluginDescribeCmd())
	cmd.AddCommand(newPluginInstallCmd())

	return cmd
}

func createPluginCommand(_ *cobra.Command, plugin *plugins.LoadedPlugin, cmdInfo *plugins.PluginCommandInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdInfo.Name,
		Short: cmdInfo.Description,
		Long:  fmt.Sprintf("%s\n\nProvided by plugin: %s v%s", cmdInfo.Description, plugin.Metadata.Name, plugin.Metadata.Version),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Collect all flags that were set
			flags := make(map[string]*plugins.SimpleFlag)

			for _, flag := range cmdInfo.Flags {
				simpleFlag := &plugins.SimpleFlag{
					Name:      flag.Name,
					Shorthand: flag.Shorthand,
					Usage:     flag.Description,
				}

				switch flag.Type {
				case stringFlagType:
					if val, err := cmd.Flags().GetString(flag.Name); err == nil {
						simpleFlag.Value = val
					}
				case boolFlagType:
					if val, err := cmd.Flags().GetBool(flag.Name); err == nil {
						simpleFlag.Value = strconv.FormatBool(val)
					}
				case intFlagType:
					if val, err := cmd.Flags().GetInt(flag.Name); err == nil {
						simpleFlag.Value = strconv.Itoa(val)
					}
				}
				flags[flag.Name] = simpleFlag
			}

			// instead of processing the persistent flags, we try to get them directly from viper
			cliConfig := plugins.ChainloopConfig{
				ControlPlaneAPI: viper.GetString(confOptions.controlplaneAPI.viperKey),
				ControlPlaneCA:  viper.GetString(confOptions.controlplaneCA.viperKey),
				CASAPI:          viper.GetString(confOptions.CASAPI.viperKey),
				CASCA:           viper.GetString(confOptions.CASCA.viperKey),
				Organization:    viper.GetString(confOptions.organization.viperKey),
				Token:           viper.GetString(confOptions.authToken.viperKey),
			}

			// Create plugin configuration with command, arguments, and flags
			config := plugins.PluginExecConfig{
				Command:         cmdInfo.Name,
				Args:            args,
				Flags:           flags,
				ChainloopConfig: cliConfig,
			}

			// execute plugin command using the action pattern
			result, err := action.NewPluginExec(actionOpts, pluginManager).Run(ctx, plugin.Metadata.Name, cmdInfo.Name, config)
			if err != nil {
				return fmt.Errorf("failed to execute plugin command: %w", err)
			}

			// handle result
			if result.Error != "" {
				return fmt.Errorf("the plugin command failed: %s", result.Error)
			}

			fmt.Print(result.Output)

			// return with appropriate exit code
			if result.ExitCode != 0 {
				os.Exit(result.ExitCode)
			}

			return nil
		},
	}

	// add flags to the command
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
		Short:   "List installed plugins",
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := action.NewPluginList(actionOpts, pluginManager).Run(context.Background())
			if err != nil {
				return err
			}

			if flagOutputFormat == formatJSON {
				type pluginInfo struct {
					Name        string `json:"name"`
					Version     string `json:"version"`
					Description string `json:"description"`
					Path        string `json:"path"`
				}

				var items []pluginInfo
				for name, plugin := range result.Plugins {
					items = append(items, pluginInfo{
						Name:        name,
						Version:     plugin.Metadata.Version,
						Description: plugin.Metadata.Description,
						Path:        plugin.Path,
					})
				}

				return encodeJSON(items)
			}

			pluginListTableOutput(result.Plugins)

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
					Name        string                       `json:"name"`
					Version     string                       `json:"version"`
					Description string                       `json:"description"`
					Path        string                       `json:"path"`
					Commands    []*plugins.PluginCommandInfo `json:"commands"`
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

func newPluginInstallCmd() *cobra.Command {
	var file string
	var location string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install a plugin",
		Long:  "Install a plugin to the plugins directory from a specified URL or local file.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			opts := &action.PluginInstallOptions{
				File:     file,
				Location: location,
			}

			result, err := action.NewPluginInstall(actionOpts, pluginManager).Run(ctx, opts)
			if err != nil {
				return fmt.Errorf("failed to install plugin: %w", err)
			}

			fmt.Printf("Plugin installed successfully to: %s\n", result.FilePath)
			return nil
		},
	}

	// Common flags
	cmd.Flags().StringVarP(&file, "file", "f", "", "URL or path to the plugin to install")
	cobra.CheckErr(cmd.MarkFlagRequired("file"))

	return cmd
}

// loadAllPlugins loads all plugins and registers their commands to the root command
func loadAllPlugins(rootCmd *cobra.Command) error {
	ctx := rootCmd.Context()

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

			pluginCmd := createPluginCommand(rootCmd, plugin, cmdInfo)
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
func pluginListTableOutput(plugins map[string]*plugins.LoadedPlugin) {
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
}

func pluginInfoTableOutput(plugin *plugins.LoadedPlugin) {
	t := newTableWriter()
	t.SetTitle(fmt.Sprintf("Plugin: %s", plugin.Metadata.Name))
	t.AppendSeparator()
	t.AppendRow(table.Row{"Version", plugin.Metadata.Version})
	t.AppendSeparator()
	t.AppendRow(table.Row{"Description", plugin.Metadata.Description})
	t.AppendSeparator()
	t.AppendRow(table.Row{"Commands", fmt.Sprintf("%d command(s)", len(plugin.Metadata.Commands))})
	t.AppendSeparator()
	t.Render()

	pluginInfoFlagsTableOutput(plugin)
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

	for _, cmd := range plugin.Metadata.Commands {
		t := newTableWriter()
		t.SetTitle(fmt.Sprintf("Command: %s, flags:", cmd.Name))
		t.AppendSeparator()

		for _, flag := range cmd.Flags {
			defaultValue := ""
			if flag.Default != nil {
				defaultValue = fmt.Sprintf("(default: %v)", flag.Default)
			}
			t.AppendRow(table.Row{fmt.Sprintf("--%s", flag.Name), flag.Description, defaultValue})
		}
		t.Render()
	}
}
