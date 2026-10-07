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

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/skill"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

// Compile-time check that Provider records skills.
var _ trace.SkillTracker = (*Provider)(nil)

// skillTool is the tool that the model calls to load a skill.
const skillTool = "Skill"

// mainTranscriptKey names the main transcript in the scan offsets of a
// session. Subagent transcripts use their file name.
const mainTranscriptKey = "main"

// commandNameRe finds the slash command of a user message, as Claude Code
// writes it: <command-name>/graphify</command-name>.
var commandNameRe = regexp.MustCompile(`<command-name>\s*/?([^<\s]+)\s*</command-name>`)

// skillRecord holds the fields of a transcript record that tell a skill use.
type skillRecord struct {
	Type       string `json:"type"`
	UUID       string `json:"uuid"`
	ParentUUID string `json:"parentUuid"`
	Timestamp  string `json:"timestamp"`
	IsMeta     bool   `json:"isMeta"`
	// SourceToolUseID links the message that holds a loaded skill to the
	// tool call that loaded it. A skill that the user loaded has none.
	SourceToolUseID string `json:"sourceToolUseID"`
	Message         *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// skillBlock is a content block of a transcript message.
type skillBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input struct {
		Skill string `json:"skill"`
	} `json:"input"`
}

// text returns the text of a message, whether the content is a string or a
// list of blocks.
func (r *skillRecord) text() string {
	if r.Message == nil {
		return ""
	}

	var s string
	if json.Unmarshal(r.Message.Content, &s) == nil {
		return s
	}

	var b strings.Builder
	for _, block := range r.blocks() {
		if block.Type == "text" {
			b.WriteString(block.Text)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (r *skillRecord) blocks() []skillBlock {
	if r.Message == nil {
		return nil
	}

	var blocks []skillBlock
	if json.Unmarshal(r.Message.Content, &blocks) != nil {
		return nil
	}

	return blocks
}

// mayHoldSkillUse is a cheap test that a scan runs before it decodes a line.
// Only the lines that start or load a skill pass it: the meta messages, the
// skill tool calls and the slash commands. The command marker is matched
// without its brackets, which a JSON encoder can escape.
func mayHoldSkillUse(line []byte) bool {
	return bytes.Contains(line, []byte(`"isMeta":true`)) ||
		bytes.Contains(line, []byte(`"name":"`+skillTool+`"`)) ||
		bytes.Contains(line, []byte("command-name"))
}

// start is the record that started a skill use: a tool call or a slash
// command.
type start struct {
	name      string
	timestamp string
}

// transcriptSkills counts the skill uses of one transcript.
//
// Claude Code writes a meta message each time it loads a skill. A meta message
// that names a skill tool call is a model start, and one whose parent is a
// slash command is a user start. A built-in command such as /clear gets no
// meta message, and a failed tool call loads nothing, so neither counts. Each
// start counts once, also when the transcript holds its meta message more than
// once.
//
// The meta message of a skill with a folder starts with "Base directory for
// this skill:". A skill built into Claude Code, such as /simplify, and a
// custom command have no folder: their use is returned with no Dir, and is
// not recorded.
func transcriptSkills(path string, subagent bool) (map[string]*trace.SkillUse, error) {
	records, err := readSkillRecords(path)
	if err != nil {
		return nil, err
	}

	toolCalls, commands := skillStarts(records)

	uses := make(map[string]*trace.SkillUse)
	counted := make(map[string]bool)
	for i := range records {
		r := &records[i]
		if r.Type != recordTypeUser || !r.IsMeta {
			continue
		}

		dir, _ := skill.BaseDir(r.text())

		s, key, byModel, ok := startOf(r, dir, toolCalls, commands)
		if !ok || counted[key] {
			continue
		}
		counted[key] = true

		at := normalizeTimestamp(s.timestamp)
		if at == "" {
			at = normalizeTimestamp(r.Timestamp)
		}

		use, ok := uses[s.name]
		if !ok {
			use = &trace.SkillUse{Name: s.name}
			uses[s.name] = use
		}
		if use.Dir == "" {
			use.Dir = dir
		}
		use.Record(at, byModel, subagent)
	}

	return uses, nil
}

// readSkillRecords returns the records of a transcript that can start or
// load a skill.
func readSkillRecords(path string) ([]skillRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var records []skillRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScannerBuffer)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !mayHoldSkillUse(line) {
			continue
		}

		var r skillRecord
		if json.Unmarshal(line, &r) != nil {
			continue
		}
		records = append(records, r)
	}

	return records, scanner.Err()
}

// skillStarts returns the skill tool calls by tool call ID, and the slash
// commands by message ID.
func skillStarts(records []skillRecord) (toolCalls, commands map[string]start) {
	toolCalls = make(map[string]start)
	commands = make(map[string]start)
	for i := range records {
		r := &records[i]
		switch r.Type {
		case recordTypeAssistant:
			for _, b := range r.blocks() {
				if b.Type == "tool_use" && b.Name == skillTool && b.ID != "" {
					toolCalls[b.ID] = start{name: strings.TrimSpace(b.Input.Skill), timestamp: r.Timestamp}
				}
			}
		case recordTypeUser:
			if r.IsMeta || r.UUID == "" {
				continue
			}
			if m := commandNameRe.FindStringSubmatch(r.text()); m != nil {
				commands[r.UUID] = start{name: m[1], timestamp: r.Timestamp}
			}
		}
	}

	return toolCalls, commands
}

// startOf returns the start of the skill use that a meta message records, and
// a key that is the same for each meta message of that one start. It reports
// false for a meta message that no start explains.
func startOf(r *skillRecord, dir string, toolCalls, commands map[string]start) (s start, key string, byModel, ok bool) {
	if r.SourceToolUseID != "" {
		s, ok = toolCalls[r.SourceToolUseID]
		if !ok && dir != "" {
			// The tool call is not in this transcript. The meta message still
			// tells that a skill loaded, and its folder names it.
			s = start{name: filepath.Base(dir), timestamp: r.Timestamp}
		}

		return s, "tool:" + r.SourceToolUseID, true, s.name != ""
	}

	s, ok = commands[r.ParentUUID]

	return s, "command:" + r.ParentUUID, false, ok && s.name != ""
}

// normalizeTimestamp returns a transcript timestamp as RFC3339 in UTC, or ""
// when it cannot be read. Claude Code writes milliseconds, and the other spec
// entries have seconds.
func normalizeTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ""
	}

	return t.UTC().Format(time.RFC3339)
}

// SessionSkills returns the skills that the session used, from the main
// transcript and the subagent transcripts copied under opts.SessionDir.
func (p *Provider) SessionSkills(opts *trace.ParseOpts) ([]trace.SkillUse, []string, error) {
	jsonlPath, err := findJSONLPath(opts.SessionDir, opts.SessionID)
	if err != nil {
		return nil, nil, err
	}

	merged, err := transcriptSkills(jsonlPath, false)
	if err != nil {
		return nil, nil, fmt.Errorf("read session file: %w", err)
	}

	var warnings []string
	subagents, _ := filepath.Glob(filepath.Join(state.RawSubagentDir(opts.SessionDir, opts.SessionID), "agent-*.jsonl"))
	for _, path := range subagents {
		uses, err := transcriptSkills(path, true)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("the skills of subagent %s were not recorded: it could not be read", filepath.Base(path)))
			continue
		}

		for name, use := range uses {
			if cur, ok := merged[name]; ok {
				cur.Merge(*use)
			} else {
				merged[name] = use
			}
		}
	}

	return trace.SortedSkillUses(merged), warnings, nil
}

// NewSkillLoads reads the transcripts of the session from where the last
// hook stopped, and returns the folder of each skill that they loaded since.
// It reads the live transcripts, not the copies in the state, because the
// hooks do not refresh the copies.
func (p *Provider) NewSkillLoads(store *state.Store, input *trace.HookInput) []string {
	projectsDir := claudeProjectsDir()
	if projectsDir == "" || !state.ValidSessionID(input.SessionID) {
		return nil
	}

	sourceDir, err := transcriptSourceDir(projectsDir, input.TranscriptPath, input.SessionID)
	if err != nil {
		if input.Cwd == "" {
			return nil
		}
		sourceDir = filepath.Join(projectsDir, encodeCWDForClaudePath(input.Cwd))
	}

	offsets, err := store.SkillScanOffsets(input.SessionID)
	if err != nil {
		offsets = map[string]int64{}
	}

	// The main transcript first, then the subagents, which Glob sorts.
	subagents, _ := filepath.Glob(filepath.Join(sourceDir, input.SessionID, "subagents", "agent-*.jsonl"))
	paths := append([]string{filepath.Join(sourceDir, input.SessionID+".jsonl")}, subagents...)

	var dirs []string
	seen := make(map[string]bool)
	changed := false
	for i, path := range paths {
		key := mainTranscriptKey
		if i > 0 {
			key = filepath.Base(path)
		}

		found, next, err := scanSkillLoads(path, offsets[key])
		if err != nil {
			continue
		}
		if next != offsets[key] {
			offsets[key] = next
			changed = true
		}
		for _, dir := range found {
			if !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
	}

	if changed {
		// A failure costs the next hook a scan of the same lines again.
		_ = store.SetSkillScanOffsets(input.SessionID, offsets)
	}

	return dirs
}

// scanSkillLoads reads path from offset and returns the skill folders that
// the complete lines name, with the offset after the last complete line. A
// line that the agent is still writing is left for the next hook.
func scanSkillLoads(path string, offset int64) ([]string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, offset, err
	}
	// A shorter file is a new file, so it is read from the start.
	if info.Size() < offset {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}

	var dirs []string
	reader := bufio.NewReaderSize(f, 64*1024)
	for {
		line, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			return dirs, offset, nil
		}
		if err != nil {
			return dirs, offset, err
		}
		offset += int64(len(line))

		if !skill.HasBaseDir(line) {
			continue
		}

		var r skillRecord
		if json.Unmarshal(line, &r) != nil || r.Type != recordTypeUser || !r.IsMeta {
			continue
		}
		if dir, ok := skill.BaseDir(r.text()); ok {
			dirs = append(dirs, dir)
		}
	}
}

// SkillRoots returns the Claude Code folders that tell where a skill came
// from.
func (p *Provider) SkillRoots(repoRoot string) skill.Roots {
	roots := skill.Roots{
		Organization: []string{managedSettingsDir()},
		Project:      []string{repoRoot},
	}

	if home, err := os.UserHomeDir(); err == nil {
		roots.Plugin = []string{filepath.Join(home, ".claude", "plugins")}
		roots.User = []string{filepath.Join(home, ".claude", "skills")}
	}

	return roots
}

// managedSettingsDir is the folder where an administrator installs the
// Claude Code configuration for all users of a machine.
func managedSettingsDir() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/ClaudeCode"
	case "windows":
		return `C:\Program Files\ClaudeCode`
	default:
		return "/etc/claude-code"
	}
}
