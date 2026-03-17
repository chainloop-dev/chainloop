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

// ConfigFileKind classifies the purpose of a discovered configuration file.
type ConfigFileKind string

const (
	// ConfigFileKindConfiguration is for settings and JSON config files.
	ConfigFileKindConfiguration ConfigFileKind = "configuration"
	// ConfigFileKindInstruction is for markdown instruction/rules files.
	ConfigFileKindInstruction ConfigFileKind = "instruction"

	// EvidenceID is the identifier for the AI agent config material type
	EvidenceID = "CHAINLOOP_AI_AGENT_CONFIG"
	// EvidenceSchemaURL is the URL to the JSON schema for AI agent config
	EvidenceSchemaURL = "https://schemas.chainloop.dev/aiagentconfig/0.1/ai-agent-config.schema.json"
)

// DiscoveredFile represents a file found during discovery, before reading its content.
type DiscoveredFile struct {
	Path string
	Kind ConfigFileKind
}

// Agent identifies the AI agent provider
type Agent struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// GitContext holds optional git information at capture time
type GitContext struct {
	Repository string `json:"repository,omitempty"`
	Branch     string `json:"branch,omitempty"`
	CommitSHA  string `json:"commit_sha,omitempty"`
}

// ConfigFile represents a single discovered configuration file
type ConfigFile struct {
	Path    string         `json:"path"`
	Kind    ConfigFileKind `json:"kind"`
	SHA256  string         `json:"sha256"`
	Size    int64          `json:"size"`
	Content string         `json:"content"`
}

// Data is the AI agent configuration payload
type Data struct {
	SchemaVersion string       `json:"schema_version"`
	Agent         Agent        `json:"agent"`
	ConfigHash    string       `json:"config_hash"`
	CapturedAt    string       `json:"captured_at"`
	GitContext    *GitContext  `json:"git_context,omitempty"`
	ConfigFiles   []ConfigFile `json:"config_files"`
	// Future fields for richer analysis
	Permissions any `json:"permissions,omitempty"`
	MCPServers  any `json:"mcp_servers,omitempty"`
	Subagents   any `json:"subagents,omitempty"`
}

// Evidence represents the complete evidence structure for AI agent config
type Evidence struct {
	ID     string `json:"chainloop.material.evidence.id"`
	Schema string `json:"schema"`
	Data   Data   `json:"data"`
}

// NewEvidence creates a new Evidence instance
func NewEvidence(data Data) *Evidence {
	return &Evidence{
		ID:     EvidenceID,
		Schema: EvidenceSchemaURL,
		Data:   data,
	}
}
