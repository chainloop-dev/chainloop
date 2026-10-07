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

import (
	"encoding/json"
	"strings"
)

const (
	// EvidenceID is the identifier for the AI coding session material type
	EvidenceID = "CHAINLOOP_AI_CODING_SESSION"
	// EvidenceSchemaURL is the URL to the JSON schema for AI coding session
	EvidenceSchemaURL = "https://schemas.chainloop.dev/aicodingsession/0.1/ai-coding-session.schema.json"
)

// How `chainloop trace` was driven for a session. Recorded so that consumers
// can tell an ongoing coding session apart from a one-shot wrapped command
// instead of presenting them as the same thing. More modes are expected, so the
// field is a plain string rather than a closed enumeration.
const (
	// ModeCoding is a session traced through the agent and git hooks installed
	// by `chainloop trace init`. It is what an absent mode means, so sessions
	// recorded before the field existed keep their meaning.
	ModeCoding = "coding"
	// ModeGeneric is a single-shot session wrapped by `chainloop trace run`.
	ModeGeneric = "generic"
)

// ResolveMode maps an unset mode onto the one an absent mode means, so that the
// default is written down once instead of at each producer and consumer.
func ResolveMode(mode string) string {
	if mode == "" {
		return ModeCoding
	}

	return mode
}

// What a captured spec entry was resolved from, as a closed vocabulary that
// consumers switch on. Unlike Mode, this value originates in a model's output
// rather than in a Chainloop release, so a value outside the set is a mistake
// to normalise away rather than a future kind to pass through. The schema
// nonetheless leaves the field an open string, so a kind added by a later CLI
// is not rejected by a control plane that predates it.
const (
	// SpecKindTicket is an issue tracker item: a Linear or Jira ticket.
	SpecKindTicket = "ticket"
	// SpecKindDocument is a written specification: a design doc, a page in a
	// vault, an RFC.
	SpecKindDocument = "document"
	// SpecKindImage is a spec given as a picture — a mockup or a screenshot —
	// of which the stored content is whatever the agent transcribed.
	SpecKindImage = "image"
	// SpecKindText is a spec stated in the session itself rather than resolved
	// from somewhere, and the fallback for anything unrecognised.
	SpecKindText = "text"
)

// SpecKindSkill is a skill that the agent used in the session. The CLI makes
// these entries from the transcript, never the agent, so ResolveSpecKind does
// not accept it: an agent that writes it in a spec file gets kind text.
const SpecKindSkill = "skill"

// ResolveSpecKind maps a captured kind onto the vocabulary above, so the
// normalisation is written down once instead of at each producer and consumer.
func ResolveSpecKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case SpecKindTicket:
		return SpecKindTicket
	case SpecKindDocument:
		return SpecKindDocument
	case SpecKindImage:
		return SpecKindImage
	default:
		return SpecKindText
	}
}

// The roles a spec source can have. The kind is the format of a source and the
// role is its purpose: a ticket can state the task, or be only background.
const (
	// SpecRoleTask is the item that states the work to do.
	SpecRoleTask = "task"
	// SpecRoleSpec is a document that defines what to build.
	SpecRoleSpec = "spec"
	// SpecRolePlan is a plan for the work that the user approved.
	SpecRolePlan = "plan"
	// SpecRoleReference is supporting material: a screenshot, a mockup, an
	// example, or a background document.
	SpecRoleReference = "reference"
)

// ResolveSpecRole maps a captured role onto the vocabulary above. Unlike the
// kind, a role is never guessed: a value outside the vocabulary gives no role,
// so the evidence holds only what the agent stated.
func ResolveSpecRole(role string) string {
	switch r := strings.ToLower(strings.TrimSpace(role)); r {
	case SpecRoleTask, SpecRoleSpec, SpecRolePlan, SpecRoleReference:
		return r
	default:
		return ""
	}
}

// SpecEntry is one source a coding session was built from: the ticket,
// document or prompt that set the task, resolved by the agent. It is what the
// work gets judged against, which no amount of diff can answer on its own.
//
// The entry is a reference. The text itself is a separate EVIDENCE material in
// the same attestation, found by Digest, so that a source shared by many
// sessions is stored once and can be fetched and verified on its own.
type SpecEntry struct {
	// Kind is one of the SpecKind* constants.
	Kind string `json:"kind"`
	// Role is one of the SpecRole* constants. Empty when the agent stated no
	// role, or one outside the vocabulary.
	Role string `json:"role,omitempty"`
	// Title is a short name for the source, as the agent wrote it.
	Title string `json:"title,omitempty"`
	// Description says in one or two sentences what the source holds, as the
	// agent wrote it.
	Description string `json:"description,omitempty"`
	// URI is where the text came from. Empty when the task was stated in the
	// session itself and there is no external source to point at.
	URI string `json:"uri,omitempty"`
	// Digest identifies the EVIDENCE material holding the spec text, as
	// "sha256:<hex>".
	Digest string `json:"digest"`
	// CapturedAt is when the agent wrote this entry, RFC3339. It comes from the
	// file's modification time, so it is the last write rather than the first,
	// and it is trivially forgeable: not a trusted timestamp. For a skill, it
	// is the time of the first use in the transcript.
	CapturedAt string `json:"captured_at"`
	// Metadata holds data that depends on the kind. Only skill entries have
	// it.
	Metadata *SpecMetadata `json:"metadata,omitempty"`
}

// Where a skill came from. The evidence never holds the path of the skill
// folder, because the path exposes local user data.
const (
	// SkillSourceProject is a skill from the repository.
	SkillSourceProject = "project"
	// SkillSourceUser is a skill from the user's agent configuration.
	SkillSourceUser = "user"
	// SkillSourceOrganization is a skill that an administrator installs for
	// all users.
	SkillSourceOrganization = "organization"
	// SkillSourcePlugin is a skill from an installed plugin.
	SkillSourcePlugin = "plugin"
	// SkillSourceUnknown is any other skill.
	SkillSourceUnknown = "unknown"
)

// SpecMetadata is the kind-specific data of a spec entry. The fields below are
// those of a skill entry. A later kind can add its own fields.
type SpecMetadata struct {
	// Source is one of the SkillSource* constants.
	Source string `json:"source,omitempty"`
	// PackageDigest identifies the EVIDENCE material that holds an archive
	// of the full skill folder, as "sha256:<hex>". Absent when the archive was
	// not uploaded, for example because it was too large.
	PackageDigest string `json:"package_digest,omitempty"`
	// InvocationCount is the number of times the session used the skill. It
	// is ByModel plus ByUser.
	InvocationCount int `json:"invocation_count"`
	// ByModel is the number of uses that the model started with a tool call.
	ByModel int `json:"by_model"`
	// ByUser is the number of uses that the user started with a slash
	// command.
	ByUser int `json:"by_user"`
	// InSubagents is the number of uses, of either start, that came from
	// subagents.
	InSubagents int `json:"in_subagents"`
}

// Agent identifies the AI agent provider.
type Agent struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// Session holds timing and identity information for the coding session.
type Session struct {
	ID   string `json:"id"`
	Slug string `json:"slug,omitempty"`
	// Mode is one of the Mode* constants. Empty means ModeCoding.
	Mode            string `json:"mode,omitempty"`
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

// LineRange represents a contiguous range of lines.
type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// FileChange represents a single file modification in the session.
type FileChange struct {
	Path         string      `json:"path"`
	Status       string      `json:"status"`
	LinesAdded   int         `json:"lines_added,omitempty"`
	LinesRemoved int         `json:"lines_removed,omitempty"`
	Attribution  string      `json:"attribution,omitempty"`
	LineRanges   []LineRange `json:"line_ranges,omitempty"`
	SessionIDs   []string    `json:"session_ids,omitempty"`
}

// CodeChanges summarizes code modifications made during the session.
type CodeChanges struct {
	FilesModified     int          `json:"files_modified,omitempty"`
	FilesCreated      int          `json:"files_created,omitempty"`
	FilesDeleted      int          `json:"files_deleted,omitempty"`
	LinesAdded        int          `json:"lines_added,omitempty"`
	LinesRemoved      int          `json:"lines_removed,omitempty"`
	AILinesAdded      int          `json:"ai_lines_added,omitempty"`
	AILinesRemoved    int          `json:"ai_lines_removed,omitempty"`
	HumanLinesAdded   int          `json:"human_lines_added,omitempty"`
	HumanLinesRemoved int          `json:"human_lines_removed,omitempty"`
	Files             []FileChange `json:"files,omitempty"`
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

// SubagentTokens holds token usage for a subagent.
type SubagentTokens struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// Subagent describes a spawned subagent within the session.
type Subagent struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Tokens      SubagentTokens `json:"tokens"`
}

// Conversation holds message count statistics.
type Conversation struct {
	TotalMessages     int `json:"total_messages,omitempty"`
	UserMessages      int `json:"user_messages,omitempty"`
	AssistantMessages int `json:"assistant_messages,omitempty"`
}

// Data is the AI coding session payload.
type Data struct {
	SchemaVersion string  `json:"schema_version"`
	Agent         Agent   `json:"agent"`
	Session       Session `json:"session"`
	// Spec is what the session was asked to build, one entry per source the
	// agent resolved. Empty for a session that captured none.
	Spec         []SpecEntry                  `json:"spec,omitempty"`
	GitContext   *GitContext                  `json:"git_context,omitempty"`
	CodeChanges  *CodeChanges                 `json:"code_changes,omitempty"`
	Model        *Model                       `json:"model,omitempty"`
	Usage        *Usage                       `json:"usage,omitempty"`
	ToolsUsed    *ToolsUsed                   `json:"tools_used,omitempty"`
	Conversation *Conversation                `json:"conversation,omitempty"`
	Subagents    []Subagent                   `json:"subagents,omitempty"`
	RawSession   map[string][]json.RawMessage `json:"raw_session,omitempty"`
	Warnings     []string                     `json:"warnings,omitempty"`
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
