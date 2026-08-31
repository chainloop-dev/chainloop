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

package claude

import "encoding/json"

type tokenUsage struct {
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	// CacheCreation5mTokens is the subset of CacheCreationInputTokens written to the 5-minute tier.
	CacheCreation5mTokens int
	// CacheCreation1hTokens is the subset of CacheCreationInputTokens written to the 1-hour tier.
	CacheCreation1hTokens int
	CacheReadInputTokens  int
}

func (t *tokenUsage) add(u *jsonlUsage) {
	t.InputTokens += u.InputTokens
	t.OutputTokens += u.OutputTokens
	t.CacheCreationInputTokens += u.CacheCreationInputTokens
	t.CacheReadInputTokens += u.CacheReadInputTokens
	if u.CacheCreation != nil {
		t.CacheCreation5mTokens += u.CacheCreation.Ephemeral5mInputTokens
		t.CacheCreation1hTokens += u.CacheCreation.Ephemeral1hInputTokens
	} else {
		// Older records without the breakdown: attribute everything to the 5-minute tier.
		t.CacheCreation5mTokens += u.CacheCreationInputTokens
	}
}

type jsonlRecord struct {
	Type      string        `json:"type"`
	Timestamp string        `json:"timestamp"`
	SessionID string        `json:"sessionId"`
	Slug      string        `json:"slug"`
	Version   string        `json:"version"`
	Cwd       string        `json:"cwd"`
	GitBranch string        `json:"gitBranch"`
	Message   *jsonlMessage `json:"message,omitempty"`
}

type jsonlMessage struct {
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"` // can be string or array
	Usage   *jsonlUsage     `json:"usage,omitempty"`
}

type jsonlUsage struct {
	InputTokens              int                     `json:"input_tokens"`
	OutputTokens             int                     `json:"output_tokens"`
	CacheCreationInputTokens int                     `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int                     `json:"cache_read_input_tokens"`
	CacheCreation            *jsonlCacheCreationInfo `json:"cache_creation,omitempty"`
}

// jsonlCacheCreationInfo mirrors the nested usage.cache_creation object emitted by recent
// Claude API responses, which splits cache writes by tier (5-minute vs 1-hour).
type jsonlCacheCreationInfo struct {
	Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens"`
	Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens"`
}

type jsonlContentBlock struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type subagentMeta struct {
	AgentType   string `json:"agentType"`
	Description string `json:"description"`
}

type sessionData struct {
	sessionID         string
	slug              string
	version           string
	cwd               string
	gitBranch         string
	tsMin             string
	tsMax             string
	tokenUsageByModel map[string]*tokenUsage
	toolCounts        map[string]int
	messageCounts     map[string]int
	modelsSeen        map[string]int
	warnings          []string
}

// claudeSessionFile matches the JSON structure in ~/.claude/sessions/<pid>.json.
type claudeSessionFile struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	StartedAt int64  `json:"startedAt"`
}

type claudeSession struct {
	sessionID string
	cwd       string
	pid       int
	isActive  bool
	jsonlPath string
	startedAt int64
}
