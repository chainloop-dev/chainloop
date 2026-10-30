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

package pi

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/skill"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"gopkg.in/yaml.v3"
)

var _ trace.SkillTracker = (*Provider)(nil)

var userSkillBlock = regexp.MustCompile(`(?s)^<skill name="([^"]+)" location="([^"]+)">\n.*\n</skill>(?:\n\n.*)?$`)

// NewSkillLoads scans the selected live branch for completed loads. The cursor
// advances over entries, not calls, so a result in a later hook can still pair
// with the original call. Generic hooks copy the skill before the next mutation.
func (p *Provider) NewSkillLoads(store *state.Store, input *trace.HookInput) []string {
	if store == nil || input == nil || !state.ValidSessionID(input.SessionID) || !filepath.IsAbs(input.TranscriptPath) {
		return nil
	}
	info, err := os.Stat(input.TranscriptPath)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	parsed, err := readSkillSession(input.TranscriptPath, input.SessionID)
	if err != nil {
		return nil
	}
	offsets, err := store.SkillScanOffsets(input.SessionID)
	if err != nil {
		offsets = make(map[string]int64)
	}
	var last int64
	for _, entry := range parsed.Selected {
		last = max(last, int64(entry.Line))
	}
	after := offsets["main"]
	if after > last {
		after = 0
	}
	uses := selectedSkills(parsed, after)
	dirs := make([]string, 0, len(uses))
	for _, use := range uses {
		dirs = append(dirs, use.Dir)
	}
	offsets["main"] = last
	if err := store.SetSkillScanOffsets(input.SessionID, offsets); err != nil {
		return nil
	}
	return dirs
}

// SessionSkills reads only Trace's private transcript copy.
func (p *Provider) SessionSkills(opts *trace.ParseOpts) ([]trace.SkillUse, []string, error) {
	if opts == nil || !state.ValidSessionID(opts.SessionID) {
		return nil, nil, errors.New("read Pi skills: invalid session")
	}
	path, err := state.FindRawSessionFile(opts.SessionDir, opts.SessionID)
	if err != nil {
		return nil, nil, err
	}
	parsed, err := readSkillSession(path, opts.SessionID)
	if err != nil {
		return nil, nil, err
	}
	return selectedSkills(parsed, 0), nil, nil
}

func readSkillSession(path, sessionID string) (*parsedSession, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("read Pi skills: transcript unavailable")
	}
	return parseJSONL(data, sessionID)
}

type skillReadCall struct {
	Path   string `json:"path"`
	Offset int    `json:"offset"`
}

func selectedSkills(parsed *parsedSession, after int64) []trace.SkillUse {
	uses := make(map[string]*trace.SkillUse)
	calls := make(map[string]skillReadCall)
	for _, entry := range parsed.Selected {
		if entry.Type != "message" {
			continue
		}
		var message struct {
			Role       string          `json:"role"`
			Content    json.RawMessage `json:"content"`
			ToolCallID string          `json:"toolCallId"`
			ToolName   string          `json:"toolName"`
			IsError    bool            `json:"isError"`
		}
		if json.Unmarshal(entry.Message, &message) != nil {
			continue
		}
		record := func(name, path string, byModel bool) {
			if int64(entry.Line) <= after || name == "" {
				return
			}
			path = skillFilePath(path, parsed.Header.Cwd)
			if path == "" || filepath.Base(path) != skill.DefinitionFile {
				return
			}
			dir := filepath.Dir(path)
			if uses[dir] == nil {
				uses[dir] = &trace.SkillUse{Name: name, Dir: dir}
			}
			uses[dir].Record(entry.Timestamp, byModel, false)
		}
		switch message.Role {
		case "user":
			if block := userSkillBlock.FindStringSubmatch(skillMessageText(message.Content)); block != nil {
				record(block[1], block[2], false)
			}
		case "assistant":
			var blocks []struct {
				Type      string          `json:"type"`
				ID        string          `json:"id"`
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if json.Unmarshal(message.Content, &blocks) != nil {
				continue
			}
			for _, block := range blocks {
				if block.Type == "toolCall" && block.ID != "" && block.Name == "read" {
					var call skillReadCall
					if json.Unmarshal(block.Arguments, &call) == nil {
						calls[block.ID] = call
					}
				}
			}
		case "toolResult":
			call, ok := calls[message.ToolCallID]
			delete(calls, message.ToolCallID)
			if !ok || message.IsError || message.ToolName != "read" || call.Offset > 1 || call.Offset < 0 {
				continue
			}
			text := strings.ReplaceAll(skillMessageText(message.Content), "\r\n", "\n")
			front, ok := strings.CutPrefix(text, "---\n")
			if !ok {
				continue
			}
			front, _, ok = strings.Cut(front, "\n---")
			var meta struct {
				Name        string `yaml:"name"`
				Description string `yaml:"description"`
			}
			if !ok || yaml.Unmarshal([]byte(front), &meta) != nil || strings.TrimSpace(meta.Description) == "" {
				continue
			}
			if meta.Name == "" {
				meta.Name = filepath.Base(filepath.Dir(call.Path))
			}
			record(meta.Name, call.Path, true)
		}
	}
	return trace.SortedSkillUses(uses)
}

func skillMessageText(content json.RawMessage) string {
	var text string
	if json.Unmarshal(content, &text) == nil {
		return text
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) != nil {
		return ""
	}
	var texts []string
	for _, block := range blocks {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
	}
	return strings.Join(texts, "\n")
}

func skillFilePath(path, cwd string) string {
	path = strings.TrimPrefix(path, "@")
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		path = filepath.Join(home, path[2:])
	}
	if !filepath.IsAbs(path) {
		if !filepath.IsAbs(cwd) {
			return ""
		}
		path = filepath.Join(cwd, path)
	}
	return filepath.Clean(path)
}

// SkillRoots follows Pi's user/project and managed package locations. Pi does
// not identify an organization scope; arbitrary external paths stay unknown.
func (p *Provider) SkillRoots(repoRoot string) skill.Roots {
	home, err := os.UserHomeDir()
	if err != nil {
		return skill.Roots{Project: []string{repoRoot}}
	}
	agentDir := os.Getenv("PI_CODING_AGENT_DIR")
	if agentDir == "" {
		agentDir = filepath.Join(home, ".pi", "agent")
	}
	agentDir = skillFilePath(agentDir, repoRoot)
	return skill.Roots{
		Plugin: []string{
			filepath.Join(agentDir, "git"), filepath.Join(agentDir, "npm", "node_modules"),
			filepath.Join(repoRoot, ".pi", "git"), filepath.Join(repoRoot, ".pi", "npm", "node_modules"),
		},
		User:    []string{filepath.Join(agentDir, "skills"), filepath.Join(home, ".agents", "skills")},
		Project: []string{repoRoot},
	}
}
