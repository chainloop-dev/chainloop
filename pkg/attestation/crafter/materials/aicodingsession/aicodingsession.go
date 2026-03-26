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

package aicodingsession

import "encoding/json"

const (
	// EvidenceID is the identifier for the AI coding session material type
	EvidenceID = "CHAINLOOP_AI_CODING_SESSION"
	// EvidenceSchemaURL is the URL to the JSON schema for AI coding session
	EvidenceSchemaURL = "https://schemas.chainloop.dev/aicodingsession/0.1/ai-coding-session.schema.json"
)

// Agent identifies the AI agent provider.
type Agent struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// Session holds timing and identity information for the coding session.
type Session struct {
	ID              string `json:"id"`
	Slug            string `json:"slug,omitempty"`
	StartedAt       string `json:"started_at"`
	EndedAt         string `json:"ended_at,omitempty"`
	DurationSeconds int    `json:"duration_seconds"`
}

// GitContext holds repository and commit information at capture time.
type GitContext struct {
	Repository  string   `json:"repository,omitempty"`
	Branch      string   `json:"branch,omitempty"`
	WorkDir     string   `json:"work_dir,omitempty"`
	CommitStart string   `json:"commit_start,omitempty"`
	CommitEnd   string   `json:"commit_end,omitempty"`
	Commits     []string `json:"commits,omitempty"`
	CommitCount int      `json:"commit_count,omitempty"`
}

// FileChange represents a single file modification in the session.
type FileChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// CodeChanges summarizes code modifications made during the session.
type CodeChanges struct {
	FilesModified int          `json:"files_modified,omitempty"`
	FilesCreated  int          `json:"files_created,omitempty"`
	FilesDeleted  int          `json:"files_deleted,omitempty"`
	LinesAdded    int          `json:"lines_added,omitempty"`
	LinesRemoved  int          `json:"lines_removed,omitempty"`
	Files         []FileChange `json:"files,omitempty"`
}

// Model holds information about the AI models used in the session.
type Model struct {
	Primary    string   `json:"primary,omitempty"`
	Provider   string   `json:"provider,omitempty"`
	ModelsUsed []string `json:"models_used,omitempty"`
}

// Usage holds token usage and cost information.
type Usage struct {
	InputTokens              int     `json:"input_tokens,omitempty"`
	OutputTokens             int     `json:"output_tokens,omitempty"`
	TotalTokens              int     `json:"total_tokens,omitempty"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens,omitempty"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd,omitempty"`
}

// ToolSummary represents usage statistics for a single tool.
type ToolSummary struct {
	ToolName        string `json:"tool_name"`
	InvocationCount int    `json:"invocation_count"`
}

// ToolsUsed summarizes tool usage during the session.
type ToolsUsed struct {
	Summary          []ToolSummary `json:"summary,omitempty"`
	TotalInvocations int           `json:"total_invocations,omitempty"`
}

// Conversation holds message count statistics.
type Conversation struct {
	TotalMessages     int `json:"total_messages,omitempty"`
	UserMessages      int `json:"user_messages,omitempty"`
	AssistantMessages int `json:"assistant_messages,omitempty"`
}

// Data is the AI coding session payload.
type Data struct {
	SchemaVersion string                       `json:"schema_version"`
	Agent         Agent                        `json:"agent"`
	Session       Session                      `json:"session"`
	GitContext    *GitContext                  `json:"git_context,omitempty"`
	CodeChanges   *CodeChanges                 `json:"code_changes,omitempty"`
	Model         *Model                       `json:"model,omitempty"`
	Usage         *Usage                       `json:"usage,omitempty"`
	ToolsUsed     *ToolsUsed                   `json:"tools_used,omitempty"`
	Conversation  *Conversation                `json:"conversation,omitempty"`
	Subagents     []json.RawMessage            `json:"subagents,omitempty"`
	RawSession    map[string][]json.RawMessage `json:"raw_session,omitempty"`
	Warnings      []string                     `json:"warnings,omitempty"`
}

// Evidence represents the complete evidence structure for AI coding session.
type Evidence struct {
	ID     string `json:"chainloop.material.evidence.id"`
	Schema string `json:"schema"`
	Data   Data   `json:"data"`
}

// NewEvidence creates a new Evidence instance.
func NewEvidence(data Data) *Evidence {
	return &Evidence{
		ID:     EvidenceID,
		Schema: EvidenceSchemaURL,
		Data:   data,
	}
}
