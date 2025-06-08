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
)

// Plugin is the interface that plugins must implement.
type Plugin interface {
	// Exec executes a command within the plugin
	Exec(ctx context.Context, command string, arguments map[string]any) (ExecResult, error)

	// GetMetadata returns plugin metadata including commands it provides
	GetMetadata(ctx context.Context) (PluginMetadata, error)
}

// ExecResult represents the result of executing a plugin command
type ExecResult interface {
	// GetOutput returns the command output
	GetOutput() string

	// GetError returns any error message
	GetError() string

	// GetExitCode returns the exit code
	GetExitCode() int

	// GetData returns any structured data
	GetData() map[string]any
}

// PluginMetadata contains information about the plugin.
type PluginMetadata struct {
	Name        string
	Version     string
	Description string
	Commands    []CommandInfo
}

// CommandInfo describes a command provided by the plugin
type CommandInfo struct {
	Name        string
	Description string
	Usage       string
	Flags       []FlagInfo
}

// FlagInfo describes a command flag
type FlagInfo struct {
	Name        string
	Shorthand   string
	Description string
	Type        string
	Default     any
	Required    bool
}
