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
	"encoding/gob"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/hashicorp/go-plugin"
)

func init() {
	// Register types that will be sent over RPC
	gob.Register(PluginExecConfig{})
	gob.Register(PluginExecResult{})
	gob.Register(PluginMetadata{})
	gob.Register(PluginCommandInfo{})
}

// Handshake is a common handshake that is shared by CLI plugins and the host.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "CHAINLOOP_CLI_PLUGIN",
	MagicCookieValue: "chainloop-cli-plugin-v1",
}

// PluginMap is the map of plugins.
var PluginMap = map[string]plugin.Plugin{
	"chainloop": &ChainloopCliPlugin{},
}

func GetPluginsDir(appName string) string {
	return filepath.Join(xdg.ConfigHome, appName, "plugins")
}
