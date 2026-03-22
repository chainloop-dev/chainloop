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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractMCPServers(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []MCPServer
		wantErr bool
	}{
		{
			name: "stdio server with command, args, and env",
			input: `{
				"mcpServers": {
					"filesystem": {
						"command": "npx",
						"args": ["-y", "@modelcontextprotocol/server-filesystem"],
						"env": {"HOME": "/home/user", "API_KEY": "sk-secret-123"}
					}
				}
			}`,
			want: []MCPServer{
				{
					Name:    "filesystem",
					Command: "npx",
					Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
					EnvKeys: []string{"API_KEY", "HOME"},
				},
			},
		},
		{
			name: "remote URL server",
			input: `{
				"mcpServers": {
					"remote": {
						"url": "https://example.com/mcp"
					}
				}
			}`,
			want: []MCPServer{
				{Name: "remote", URL: "https://example.com/mcp"},
			},
		},
		{
			name: "disabled server",
			input: `{
				"mcpServers": {
					"disabled-server": {
						"command": "node",
						"args": ["server.js"],
						"disabled": true
					}
				}
			}`,
			want: []MCPServer{
				{
					Name:     "disabled-server",
					Command:  "node",
					Args:     []string{"server.js"},
					Disabled: true,
				},
			},
		},
		{
			name: "multiple servers sorted by name",
			input: `{
				"mcpServers": {
					"zeta": {"command": "zeta-cmd"},
					"alpha": {"command": "alpha-cmd"}
				}
			}`,
			want: []MCPServer{
				{Name: "alpha", Command: "alpha-cmd"},
				{Name: "zeta", Command: "zeta-cmd"},
			},
		},
		{
			name: "env keys sorted alphabetically",
			input: `{
				"mcpServers": {
					"srv": {
						"command": "cmd",
						"env": {"ZEBRA": "z", "APPLE": "a", "MANGO": "m"}
					}
				}
			}`,
			want: []MCPServer{
				{
					Name:    "srv",
					Command: "cmd",
					EnvKeys: []string{"APPLE", "MANGO", "ZEBRA"},
				},
			},
		},
		{
			name:  "no mcpServers key",
			input: `{"other": "value"}`,
			want:  nil,
		},
		{
			name:  "empty mcpServers",
			input: `{"mcpServers": {}}`,
			want:  nil,
		},
		{
			name:    "invalid JSON",
			input:   `not json`,
			wantErr: true,
		},
		{
			name: "settings.json with mcpServers among other keys",
			input: `{
				"permissions": {"allow": ["read"]},
				"mcpServers": {
					"my-server": {"command": "my-cmd", "args": ["--flag"]}
				},
				"theme": "dark"
			}`,
			want: []MCPServer{
				{Name: "my-server", Command: "my-cmd", Args: []string{"--flag"}},
			},
		},
		{
			name: "env values are stripped",
			input: `{
				"mcpServers": {
					"srv": {
						"command": "cmd",
						"env": {"SECRET": "super-secret-value", "TOKEN": "bearer-xyz"}
					}
				}
			}`,
			want: []MCPServer{
				{
					Name:    "srv",
					Command: "cmd",
					EnvKeys: []string{"SECRET", "TOKEN"},
				},
			},
		},
		{
			name: "server with no env",
			input: `{
				"mcpServers": {
					"simple": {"command": "echo"}
				}
			}`,
			want: []MCPServer{
				{Name: "simple", Command: "echo"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractMCPServers([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
