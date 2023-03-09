//go:build tools
// +build tools

// following https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

package main

import (
	_ "github.com/google/wire/cmd/wire"

	// Indirectly used by wire
	_ "github.com/google/subcommands"
)
