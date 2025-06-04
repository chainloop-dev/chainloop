package plugins

import (
	"encoding/gob"

	"github.com/hashicorp/go-plugin"
)

func init() {
	// Register types that will be sent over RPC
	gob.Register(map[string]any{})
	gob.Register([]any{})
	gob.Register(ExecResponse{})
	gob.Register(PluginMetadata{})
	gob.Register(CommandInfo{})
	gob.Register(FlagInfo{})
	gob.Register([]CommandInfo{})
	gob.Register([]FlagInfo{})
	gob.Register([]string{})
	gob.Register([]map[string]any{})
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
