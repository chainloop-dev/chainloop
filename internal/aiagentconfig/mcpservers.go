//
// Copyright 2026 The Chainloop Authors.
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

package aiagentconfig

import (
	"encoding/json"
	"maps"
	"slices"
	"sort"
)

// rawMCPConfig represents the top-level structure containing MCP server definitions.
// Both .mcp.json and .claude/settings.json use "mcpServers" as the key.
type rawMCPConfig struct {
	MCPServers map[string]rawMCPServerEntry `json:"mcpServers"`
}

// rawMCPServerEntry is the raw JSON shape of a single MCP server entry.
type rawMCPServerEntry struct {
	Command  string            `json:"command,omitempty"`
	Args     []string          `json:"args,omitempty"`
	URL      string            `json:"url,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`
}

// ExtractMCPServers parses MCP server entries from raw JSON content.
// It handles both .mcp.json format and .claude/settings.json format.
// Environment variable values are stripped; only key names are retained.
// Returns nil without error if the JSON is valid but contains no mcpServers.
func ExtractMCPServers(content []byte) ([]MCPServer, error) {
	var raw rawMCPConfig
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, err
	}

	if len(raw.MCPServers) == 0 {
		return nil, nil
	}

	servers := make([]MCPServer, 0, len(raw.MCPServers))
	for name, entry := range raw.MCPServers {
		srv := MCPServer{
			Name:     name,
			Command:  entry.Command,
			Args:     entry.Args,
			URL:      entry.URL,
			Disabled: entry.Disabled,
		}

		if len(entry.Env) > 0 {
			srv.EnvKeys = slices.Sorted(maps.Keys(entry.Env))
		}

		servers = append(servers, srv)
	}

	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})

	return servers, nil
}
