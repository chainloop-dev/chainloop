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

package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

type tokenUsage struct {
	Input      int `json:"input_tokens"`
	Cached     int `json:"cached_input_tokens"`
	CacheWrite int `json:"cache_write_input_tokens"`
	Output     int `json:"output_tokens"`
	Total      int `json:"total_tokens"`
}

type rolloutEntry struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type rolloutPayload struct {
	ID            string      `json:"id"`
	Type          string      `json:"type"`
	Version       string      `json:"cli_version"`
	ModelProvider string      `json:"model_provider"`
	Model         string      `json:"model"`
	Role          string      `json:"role"`
	Name          string      `json:"name"`
	CallID        string      `json:"call_id"`
	ResponseID    string      `json:"response_id"`
	Usage         *tokenUsage `json:"usage"`
	ThreadUsage   *tokenUsage `json:"thread_token_usage"`
	Info          *struct {
		Total *tokenUsage `json:"total_token_usage"`
	} `json:"info"`
}

// ParseSession reads only the private rollout copy. Unknown record types stay
// in the raw transcript, because the Codex rollout format can evolve.
func (p *Provider) ParseSession(ctx context.Context, opts *trace.ParseOpts) (*aicodingsession.Evidence, error) {
	if opts == nil || !state.ValidSessionID(opts.SessionID) {
		return nil, errors.New("parse Codex session: invalid options")
	}
	path, err := state.FindRawSessionFile(opts.SessionDir, opts.SessionID)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d := aicodingsession.Data{
		SchemaVersion: "v2",
		Agent:         aicodingsession.Agent{Name: Name, Version: opts.AgentVersion},
		Session:       aicodingsession.Session{ID: opts.SessionID},
		Model:         &aicodingsession.Model{Primary: opts.Model},
		ToolsUsed:     &aicodingsession.ToolsUsed{},
		Conversation:  &aicodingsession.Conversation{},
		RawSession:    map[string][]json.RawMessage{"main": {}},
	}
	parser := rolloutParser{
		data:         &d,
		models:       map[string]int{},
		tools:        map[string]int{},
		calls:        map[string]bool{},
		usageByModel: map[string]tokenUsage{},
		model:        opts.Model,
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	line := 0
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line++
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var e rolloutEntry
		var payload rolloutPayload
		if json.Unmarshal(raw, &e) != nil || e.Type == "" || len(e.Payload) == 0 || json.Unmarshal(e.Payload, &payload) != nil {
			d.Warnings = append(d.Warnings, fmt.Sprintf("invalid Codex rollout record on line %d", line))
			continue
		}
		entry, err := transcriptEntry(e, payload, append(json.RawMessage(nil), raw...), parser.model, line)
		if err != nil {
			return nil, err
		}
		d.RawSession["main"] = append(d.RawSession["main"], entry)
		if err := parser.consume(e, payload, line); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Codex rollout: %w", err)
	}
	if err := parser.finish(); err != nil {
		return nil, err
	}

	return aicodingsession.NewEvidence(d), nil
}

type rolloutParser struct {
	data                  *aicodingsession.Data
	models                map[string]int
	tools                 map[string]int
	calls                 map[string]bool
	usageByModel          map[string]tokenUsage
	model                 string
	first, last           time.Time
	previous              tokenUsage
	usageRecords          []usageRecord
	haveUsage, haveHeader bool
}

// Keep the model at the time of the record. Source selection happens after
// reading the rollout so a legacy snapshot before the first native record
// cannot be counted in addition to native usage.
type usageRecord struct {
	model      string
	line       int
	native     bool
	responseID string
	usage      *tokenUsage
	total      *tokenUsage
}

func (p *rolloutParser) consume(e rolloutEntry, payload rolloutPayload, line int) error {
	if at, err := time.Parse(time.RFC3339Nano, e.Timestamp); err == nil {
		if p.first.IsZero() || at.Before(p.first) {
			p.first = at
		}
		if p.last.IsZero() || at.After(p.last) {
			p.last = at
		}
	} else {
		p.data.Warnings = append(p.data.Warnings, fmt.Sprintf("invalid Codex timestamp on line %d", line))
	}
	switch e.Type {
	case "session_meta":
		if payload.ID != p.data.Session.ID {
			return errors.New("parse Codex session: rollout session ID mismatch")
		}
		p.haveHeader = true
		p.data.Agent.Version = payload.Version
		p.data.Model.Provider = payload.ModelProvider
	case "turn_context":
		if payload.Model != "" {
			p.model = payload.Model
			if _, ok := p.models[p.model]; !ok {
				p.models[p.model] = 0
			}
		}
	case "response_item":
		switch payload.Type {
		case "message":
			switch payload.Role {
			case "user":
				p.data.Conversation.UserMessages++
			case "assistant":
				p.data.Conversation.AssistantMessages++
				if p.model != "" {
					p.models[p.model]++
				}
			}
		case "function_call", "custom_tool_call":
			if payload.Name != "" && (payload.CallID == "" || !p.calls[payload.CallID]) {
				p.tools[payload.Name]++
				p.calls[payload.CallID] = true
			}
		}
	case "event_msg":
		if payload.Type == "token_count" && payload.Info != nil && payload.Info.Total != nil {
			p.usageRecords = append(p.usageRecords, usageRecord{model: p.model, line: line, total: payload.Info.Total})
		}
	case "token_usage_record":
		if payload.Usage != nil || payload.ThreadUsage != nil {
			p.usageRecords = append(p.usageRecords, usageRecord{
				model: p.model, line: line, native: true,
				responseID: payload.ResponseID, usage: payload.Usage, total: payload.ThreadUsage,
			})
		}
	}

	return nil
}

func (p *rolloutParser) consumeSelectedUsage() {
	haveNative, useResponses := false, true
	for _, r := range p.usageRecords {
		if r.native {
			haveNative = true
			useResponses = useResponses && r.responseID != "" && r.usage != nil
		}
	}
	seen := make(map[string]bool)
	for _, r := range p.usageRecords {
		if r.native != haveNative {
			continue
		}
		if r.native && useResponses && seen[r.responseID] {
			continue
		}
		var u *tokenUsage
		if r.native && useResponses {
			u = r.usage
		} else {
			u = r.total
		}
		if u == nil {
			p.data.Warnings = append(p.data.Warnings, fmt.Sprintf("missing Codex cumulative token usage on line %d", r.line))
			continue
		}
		if !p.validUsage(*u, r.line) {
			continue
		}
		if r.native && useResponses {
			seen[r.responseID] = true
		}
		if r.native && useResponses {
			p.addUsage(r.model, *u)
		} else {
			p.consumeUsage(r.model, *u, r.line)
		}
	}
}

func (p *rolloutParser) validUsage(total tokenUsage, line int) bool {
	if total.Input < 0 || total.Output < 0 || total.Cached < 0 || total.CacheWrite < 0 || total.Total < 0 {
		p.data.Warnings = append(p.data.Warnings, fmt.Sprintf("negative Codex token count on line %d", line))
		return false
	}
	if total.Cached > total.Input || total.CacheWrite > total.Input-total.Cached {
		p.data.Warnings = append(p.data.Warnings, fmt.Sprintf("Codex cache tokens exceed input tokens on line %d", line))
		return false
	}
	return true
}

func (p *rolloutParser) consumeUsage(model string, total tokenUsage, line int) {
	// Counters are session totals, not increments. Repeated snapshots add zero.
	delta := tokenUsage{
		Input:      total.Input - p.previous.Input,
		Cached:     total.Cached - p.previous.Cached,
		CacheWrite: total.CacheWrite - p.previous.CacheWrite,
		Output:     total.Output - p.previous.Output,
		Total:      total.Total - p.previous.Total,
	}
	if delta.Input < 0 || delta.Output < 0 || delta.Cached < 0 || delta.CacheWrite < 0 || delta.Total < 0 {
		// A lower snapshot alone cannot establish a reset. Keep the previous
		// high-water mark instead of adding a whole session again. Native
		// per-response records, when available, handle real counter resets.
		p.data.Warnings = append(p.data.Warnings, fmt.Sprintf("Codex cumulative token counters decreased on line %d; ignoring snapshot", line))
		return
	}
	if !p.validUsage(delta, line) {
		return
	}
	p.addUsage(model, delta)
	p.previous = total
}

func (p *rolloutParser) addUsage(model string, delta tokenUsage) {
	u := p.usageByModel[model]
	u.Input += delta.Input
	u.Cached += delta.Cached
	u.CacheWrite += delta.CacheWrite
	u.Output += delta.Output
	u.Total += delta.Total
	p.usageByModel[model] = u
	p.haveUsage = true
}

func (p *rolloutParser) finish() error {
	if !p.haveHeader {
		return errors.New("parse Codex session: missing session metadata")
	}
	p.consumeSelectedUsage()
	if !p.first.IsZero() {
		p.data.Session.StartedAt = p.first.UTC().Format(time.RFC3339)
		p.data.Session.EndedAt = p.last.UTC().Format(time.RFC3339)
		p.data.Session.DurationSeconds = int(p.last.Sub(p.first).Seconds())
	}
	best := -1
	for name := range p.models {
		p.data.Model.ModelsUsed = append(p.data.Model.ModelsUsed, name)
	}
	sort.Strings(p.data.Model.ModelsUsed)
	for _, name := range p.data.Model.ModelsUsed {
		if p.models[name] > best {
			p.data.Model.Primary = name
			best = p.models[name]
		}
	}
	if p.data.Model.Primary == "" {
		p.data.Warnings = append(p.data.Warnings, "Codex rollout has no model information")
	}
	for name, count := range p.tools {
		p.data.ToolsUsed.Summary = append(p.data.ToolsUsed.Summary, aicodingsession.ToolSummary{ToolName: name, InvocationCount: count})
		p.data.ToolsUsed.TotalInvocations += count
	}
	sort.Slice(p.data.ToolsUsed.Summary, func(i, j int) bool {
		a, b := p.data.ToolsUsed.Summary[i], p.data.ToolsUsed.Summary[j]
		if a.InvocationCount != b.InvocationCount {
			return a.InvocationCount > b.InvocationCount
		}
		return a.ToolName < b.ToolName
	})
	p.data.Conversation.TotalMessages = p.data.Conversation.UserMessages + p.data.Conversation.AssistantMessages
	if p.haveUsage {
		p.data.Usage = &aicodingsession.Usage{}
		// Stable ordering also keeps warnings deterministic for sessions using several models.
		names := make([]string, 0, len(p.usageByModel))
		for name := range p.usageByModel {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			u := p.usageByModel[name]
			// Match Claude's evidence convention: input and total exclude
			// cache, which remains in separate fields and in the cost estimate.
			u.Input -= u.Cached + u.CacheWrite
			u.Total = u.Input + u.Output
			p.data.Usage.InputTokens += u.Input
			p.data.Usage.OutputTokens += u.Output
			p.data.Usage.TotalTokens += u.Total
			p.data.Usage.CacheReadInputTokens += u.Cached
			p.data.Usage.CacheCreationInputTokens += u.CacheWrite
			p.data.Usage.EstimatedCostUSD += estimateCost(name, u, &p.data.Warnings)
		}
		p.data.Usage.EstimatedCostUSD = roundCost(p.data.Usage.EstimatedCostUSD)
	} else {
		p.data.Warnings = append(p.data.Warnings, "Codex rollout has no token usage; cost is unavailable")
	}
	return nil
}
