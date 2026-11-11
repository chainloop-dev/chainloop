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

package copilot

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

// Event types of the Copilot session event log.
const (
	eventSessionStartRecord = "session.start"
	eventModelChange        = "session.model_change"
	eventUserMessage        = "user.message"
	eventAssistantMessage   = "assistant.message"
	eventToolComplete       = "tool.execution_complete"
	eventUsageRecord        = "session.usage_record"
	eventShutdown           = "session.shutdown"

	// modelProvider is the vendor of the models as Trace records it: Copilot
	// routes requests to models of several vendors.
	modelProvider = "github-copilot"
)

var knownEvents = map[string]bool{
	eventSessionStartRecord: true,
	eventModelChange:        true,
	eventUserMessage:        true,
	eventAssistantMessage:   true,
	eventToolComplete:       true,
	eventUsageRecord:        true,
	eventShutdown:           true,
}

// tokenUsage is the usage of one model. Input excludes the cache tokens.
type tokenUsage struct {
	Input, Output, CacheRead, CacheWrite int
}

func (u *tokenUsage) add(o tokenUsage) {
	u.Input += o.Input
	u.Output += o.Output
	u.CacheRead += o.CacheRead
	u.CacheWrite += o.CacheWrite
}

// copilotUsage is the usage form of the event log. Its input tokens include
// the cache tokens.
type copilotUsage struct {
	Model            string `json:"model"`
	InputTokens      int    `json:"inputTokens"`
	OutputTokens     int    `json:"outputTokens"`
	CacheReadTokens  int    `json:"cacheReadTokens"`
	CacheWriteTokens int    `json:"cacheWriteTokens"`
	APICallID        string `json:"apiCallId"`
}

func (u copilotUsage) normalized() tokenUsage {
	return tokenUsage{
		Input:      max(u.InputTokens-u.CacheReadTokens-u.CacheWriteTokens, 0),
		Output:     u.OutputTokens,
		CacheRead:  u.CacheReadTokens,
		CacheWrite: u.CacheWriteTokens,
	}
}

type logEvent struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Timestamp string          `json:"timestamp"`
	AgentID   string          `json:"agentId"`
	Data      json.RawMessage `json:"data"`
}

// eventData holds the fields of the event types that the parser reads.
type eventData struct {
	SessionID      string `json:"sessionId"`
	CopilotVersion string `json:"copilotVersion"`
	SelectedModel  string `json:"selectedModel"`
	NewModel       string `json:"newModel"`
	Model          string `json:"model"`
	Content        string `json:"content"`
	ReasoningText  string `json:"reasoningText"`
	OutputTokens   int    `json:"outputTokens"`
	ToolCallID     string `json:"toolCallId"`
	ToolRequests   []struct {
		ToolCallID string          `json:"toolCallId"`
		Name       string          `json:"name"`
		Arguments  json.RawMessage `json:"arguments"`
	} `json:"toolRequests"`
	Result *struct {
		Content string `json:"content"`
	} `json:"result"`
	Usage        *copilotUsage `json:"usage"`
	CurrentModel string        `json:"currentModel"`
	ModelMetrics map[string]struct {
		Usage copilotUsage `json:"usage"`
	} `json:"modelMetrics"`
}

// ParseSession reads only the private copy of the event log. Unknown event
// types stay in the raw session, because the format can change.
func (p *Provider) ParseSession(ctx context.Context, opts *trace.ParseOpts) (*aicodingsession.Evidence, error) {
	if opts == nil || !state.ValidSessionID(opts.SessionID) {
		return nil, errors.New("parse Copilot session: invalid options")
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
		Model:         &aicodingsession.Model{Primary: opts.Model, Provider: modelProvider},
		ToolsUsed:     &aicodingsession.ToolsUsed{},
		Conversation:  &aicodingsession.Conversation{},
		RawSession:    map[string][]json.RawMessage{"main": {}},
	}
	parser := newLogParser(&d)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxHookPayloadBytes)
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
		raw = append(json.RawMessage(nil), raw...)
		var e logEvent
		if json.Unmarshal(raw, &e) != nil || e.Type == "" {
			d.Warnings = append(d.Warnings, fmt.Sprintf("invalid Copilot event on line %d", line))
			continue
		}
		// Only the event types that the parser reads are decoded. Other
		// types keep any shape, and stay in the raw session as they are.
		var data eventData
		if knownEvents[e.Type] && len(e.Data) > 0 && json.Unmarshal(e.Data, &data) != nil {
			d.Warnings = append(d.Warnings, fmt.Sprintf("unreadable Copilot %s event on line %d", e.Type, line))
			d.RawSession["main"] = append(d.RawSession["main"], json.RawMessage(raw))
			continue
		}
		d.RawSession["main"] = append(d.RawSession["main"], rawEntry(e, data, raw, line))
		if err := parser.consume(e, data, line); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Copilot event log: %w", err)
	}
	if err := parser.finish(); err != nil {
		return nil, err
	}
	return aicodingsession.NewEvidence(d), nil
}

type usageRecord struct {
	line  int
	model string
	usage tokenUsage
}

type logParser struct {
	data         *aicodingsession.Data
	models       map[string]int
	tools        map[string]int
	calls        map[string]bool
	model        string
	first, last  time.Time
	haveHeader   bool
	shutdowns    map[string]tokenUsage
	lastShutdown int
	records      map[string]usageRecord
	messageUsage map[string]tokenUsage
}

func newLogParser(d *aicodingsession.Data) *logParser {
	return &logParser{
		data:         d,
		models:       map[string]int{},
		tools:        map[string]int{},
		calls:        map[string]bool{},
		model:        d.Model.Primary,
		shutdowns:    map[string]tokenUsage{},
		records:      map[string]usageRecord{},
		messageUsage: map[string]tokenUsage{},
	}
}

func (p *logParser) noteModel(model string) {
	if model == "" {
		return
	}
	p.model = model
	if _, ok := p.models[model]; !ok {
		p.models[model] = 0
	}
}

func (p *logParser) consume(e logEvent, data eventData, line int) error {
	if at, err := time.Parse(time.RFC3339Nano, e.Timestamp); err == nil {
		if p.first.IsZero() || at.Before(p.first) {
			p.first = at
		}
		if p.last.IsZero() || at.After(p.last) {
			p.last = at
		}
	}

	switch e.Type {
	case eventSessionStartRecord:
		if data.SessionID != p.data.Session.ID {
			return errors.New("parse Copilot session: session ID mismatch")
		}
		p.haveHeader = true
		if data.CopilotVersion != "" {
			p.data.Agent.Version = data.CopilotVersion
		}
		p.noteModel(data.SelectedModel)
	case eventModelChange:
		p.noteModel(data.NewModel)
	case eventUserMessage:
		if e.AgentID == "" {
			p.data.Conversation.UserMessages++
		}
	case eventAssistantMessage:
		p.consumeAssistantMessage(e, data)
	case eventToolComplete:
		p.noteModel(data.Model)
	case eventUsageRecord:
		p.consumeUsageRecord(data, line)
	case eventShutdown:
		p.consumeShutdown(e, data, line)
	}
	return nil
}

func (p *logParser) consumeAssistantMessage(e logEvent, data eventData) {
	// Newer Copilot releases name the model on each message.
	p.noteModel(data.Model)
	if e.AgentID == "" {
		p.data.Conversation.AssistantMessages++
	}
	if p.model != "" {
		p.models[p.model]++
	}
	for _, req := range data.ToolRequests {
		if req.Name == "" || (req.ToolCallID != "" && p.calls[req.ToolCallID]) {
			continue
		}
		p.tools[req.Name]++
		if req.ToolCallID != "" {
			p.calls[req.ToolCallID] = true
		}
	}
	if data.OutputTokens > 0 {
		u := p.messageUsage[p.model]
		u.Output += data.OutputTokens
		p.messageUsage[p.model] = u
	}
}

func (p *logParser) consumeUsageRecord(data eventData, line int) {
	if data.Usage == nil {
		return
	}
	p.noteModel(data.Usage.Model)
	key := data.Usage.APICallID
	if key == "" {
		key = fmt.Sprintf("line-%d", line)
	}
	// Copilot can write a record again; the latest copy wins.
	p.records[key] = usageRecord{line: line, model: data.Usage.Model, usage: data.Usage.normalized()}
}

// consumeShutdown adds a session summary. Only the main agent writes one,
// and its totals include the subagents. Each Copilot process writes one for
// its own segment of a resumed session.
func (p *logParser) consumeShutdown(e logEvent, data eventData, line int) {
	if e.AgentID != "" || len(data.ModelMetrics) == 0 {
		return
	}
	for model, m := range data.ModelMetrics {
		u := p.shutdowns[model]
		u.add(m.Usage.normalized())
		p.shutdowns[model] = u
		if _, ok := p.models[model]; !ok {
			p.models[model] = 0
		}
	}
	p.lastShutdown = line
	if data.CurrentModel != "" {
		p.data.Model.Primary = data.CurrentModel
	}
}

// usageByModel selects the token source: the shutdown totals, plus the
// records after the last shutdown; else the per-call records; else the
// output tokens of the messages, with a warning.
func (p *logParser) usageByModel() map[string]tokenUsage {
	switch {
	case len(p.shutdowns) > 0:
		out := map[string]tokenUsage{}
		for model, u := range p.shutdowns {
			out[model] = u
		}
		for _, r := range p.records {
			if r.line > p.lastShutdown {
				u := out[r.model]
				u.add(r.usage)
				out[r.model] = u
			}
		}
		return out
	case len(p.records) > 0:
		out := map[string]tokenUsage{}
		for _, r := range p.records {
			u := out[r.model]
			u.add(r.usage)
			out[r.model] = u
		}
		p.data.Warnings = append(p.data.Warnings, "Copilot event log has no session summary; token usage comes from the records of each call")
		return out
	case len(p.messageUsage) > 0:
		p.data.Warnings = append(p.data.Warnings, "Copilot event log has no session summary; token usage has only the output tokens of each message")
		return p.messageUsage
	}
	return nil
}

func (p *logParser) finish() error {
	if !p.haveHeader {
		return errors.New("parse Copilot session: missing session.start event")
	}
	d := p.data

	if !p.first.IsZero() {
		d.Session.StartedAt = p.first.UTC().Format(time.RFC3339)
		d.Session.EndedAt = p.last.UTC().Format(time.RFC3339)
		d.Session.DurationSeconds = int(p.last.Sub(p.first).Seconds())
	}

	for name := range p.models {
		d.Model.ModelsUsed = append(d.Model.ModelsUsed, name)
	}
	sort.Strings(d.Model.ModelsUsed)
	if d.Model.Primary == "" {
		best := -1
		for _, name := range d.Model.ModelsUsed {
			if p.models[name] > best {
				d.Model.Primary = name
				best = p.models[name]
			}
		}
	}
	if d.Model.Primary == "" {
		d.Warnings = append(d.Warnings, "Copilot event log has no model information")
	}

	for name, count := range p.tools {
		d.ToolsUsed.Summary = append(d.ToolsUsed.Summary, aicodingsession.ToolSummary{ToolName: name, InvocationCount: count})
		d.ToolsUsed.TotalInvocations += count
	}
	sort.Slice(d.ToolsUsed.Summary, func(i, j int) bool {
		a, b := d.ToolsUsed.Summary[i], d.ToolsUsed.Summary[j]
		if a.InvocationCount != b.InvocationCount {
			return a.InvocationCount > b.InvocationCount
		}
		return a.ToolName < b.ToolName
	})
	d.Conversation.TotalMessages = d.Conversation.UserMessages + d.Conversation.AssistantMessages

	usage := p.usageByModel()
	if len(usage) == 0 {
		d.Warnings = append(d.Warnings, "Copilot event log has no token usage; cost is unavailable")
		return nil
	}
	names := make([]string, 0, len(usage))
	for name := range usage {
		names = append(names, name)
	}
	sort.Strings(names)
	d.Usage = &aicodingsession.Usage{}
	for _, name := range names {
		u := usage[name]
		d.Usage.InputTokens += u.Input
		d.Usage.OutputTokens += u.Output
		d.Usage.TotalTokens += u.Input + u.Output
		d.Usage.CacheReadInputTokens += u.CacheRead
		d.Usage.CacheCreationInputTokens += u.CacheWrite
		model := name
		if model == "" {
			model = d.Model.Primary
		}
		d.Usage.EstimatedCostUSD += estimateCost(model, u, &d.Warnings)
	}
	d.Usage.EstimatedCostUSD = roundCost(d.Usage.EstimatedCostUSD)
	return nil
}

// toolInput returns the arguments of a tool call as a JSON object, which the
// session viewer expects. Arguments that are not an object, such as the
// patch text of apply_patch, go in an input field, as for Codex.
func toolInput(args json.RawMessage) json.RawMessage {
	if len(args) == 0 {
		return json.RawMessage(`{}`)
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(args, &object) == nil && object != nil {
		return args
	}
	wrapped, _ := json.Marshal(map[string]json.RawMessage{"input": args})
	return wrapped
}

// rawEntry converts the messages and tool results to the shared line shape,
// so the session viewer can show them, and keeps the native event in
// source_event. Other events stay as Copilot wrote them.
func rawEntry(e logEvent, data eventData, raw json.RawMessage, line int) json.RawMessage {
	entry := trace.RawSessionEntry{
		UUID:        e.ID,
		Timestamp:   e.Timestamp,
		SourceEvent: raw,
	}
	if entry.UUID == "" {
		entry.UUID = fmt.Sprintf("copilot-event-%d", line)
	}

	var blocks []any
	switch e.Type {
	case eventUserMessage:
		entry.Type = trace.RoleUser
		blocks = append(blocks, trace.RawSessionTextBlock{Type: "text", Text: data.Content})
	case eventAssistantMessage:
		entry.Type = trace.RoleAssistant
		if data.ReasoningText != "" {
			blocks = append(blocks, map[string]string{"type": "thinking", "thinking": data.ReasoningText})
		}
		if data.Content != "" {
			blocks = append(blocks, trace.RawSessionTextBlock{Type: "text", Text: data.Content})
		}
		for _, req := range data.ToolRequests {
			blocks = append(blocks, trace.RawSessionToolUseBlock{Type: "tool_use", ID: req.ToolCallID, Name: req.Name, Input: toolInput(req.Arguments)})
		}
	case eventToolComplete:
		entry.Type = trace.RoleUser
		content := ""
		if data.Result != nil {
			content = data.Result.Content
		}
		encoded, _ := json.Marshal(content)
		blocks = append(blocks, trace.RawSessionToolResultBlock{Type: "tool_result", ToolUseID: data.ToolCallID, Content: encoded})
	default:
		return raw
	}
	entry.Message.Role = entry.Type
	entry.Message.Content, _ = json.Marshal(blocks)
	return trace.MustRawSessionEntry(entry)
}
