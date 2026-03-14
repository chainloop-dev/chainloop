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
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
	Content string `json:"content"`
}

// Evidence is the AI agent configuration payload
type Evidence struct {
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
