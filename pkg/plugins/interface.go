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
