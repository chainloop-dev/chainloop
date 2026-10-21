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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/skill"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"gopkg.in/yaml.v3"
)

var _ trace.SkillTracker = (*Provider)(nil)

var skillPathPattern = regexp.MustCompile(`[^\s"'` + "`" + `<>\\]*SKILL\.md`)

type skillCall struct {
	paths []string
	cwd   string
}

// SessionSkills reads the private rollout copy, never the current agent file.
func (p *Provider) SessionSkills(opts *trace.ParseOpts) ([]trace.SkillUse, []string, error) {
	if opts == nil || !state.ValidSessionID(opts.SessionID) {
		return nil, nil, errors.New("read Codex skills: invalid session")
	}
	path, err := state.FindRawSessionFile(opts.SessionDir, opts.SessionID)
	if err != nil {
		return nil, nil, err
	}
	uses, _, err := scanCodexSkills(path, opts.SessionID, 0, false)
	return uses, nil, err
}

// NewSkillLoads recognizes newly completed loads in the live rollout. It
// validates storage and session identity before returning any skill folder.
// Pending tool calls are rescanned so a result in the next hook can pair with
// its original call. The offset counts completed uses, not pending calls.
func (p *Provider) NewSkillLoads(store *state.Store, input *trace.HookInput) []string {
	if store == nil || input == nil || !state.ValidSessionID(input.SessionID) {
		return nil
	}
	home := codexHome()
	if rec, err := store.LoadSessionRecord(input.SessionID); err == nil && rec != nil && rec.CodexStorageRoot != "" {
		home = rec.CodexStorageRoot
	}
	if home == "" {
		return nil
	}
	roots := []string{filepath.Join(home, "sessions"), filepath.Join(home, "archived_sessions")}
	path := input.TranscriptPath
	if path == "" {
		path, _ = findRollout(roots, input.SessionID)
	}
	if !filepath.IsAbs(path) || !insideStorage(path, roots) {
		return nil
	}
	offsets, err := store.SkillScanOffsets(input.SessionID)
	if err != nil {
		offsets = map[string]int64{}
	}
	uses, next, err := scanCodexSkills(path, input.SessionID, offsets[mainTranscriptKey], true)
	if err != nil {
		return nil
	}
	offsets[mainTranscriptKey] = next
	_ = store.SetSkillScanOffsets(input.SessionID, offsets)
	dirs := make([]string, 0, len(uses))
	for _, use := range uses {
		dirs = append(dirs, use.Dir)
	}
	return dirs
}

func scanCodexSkills(path, sessionID string, after int64, live bool) ([]trace.SkillUse, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, after, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, after, errors.New("read Codex skills: not a regular rollout")
	}
	if info.Size() < after {
		after = 0
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxHookPayloadBytes)
	if live {
		scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
			if atEOF && !bytes.ContainsRune(data, '\n') {
				return 0, nil, nil
			}
			return bufio.ScanLines(data, atEOF)
		})
	}
	uses := map[string]*trace.SkillUse{}
	calls := map[string]skillCall{}
	cwd := ""
	var offset int64
	first := true
	for scanner.Scan() {
		line := scanner.Bytes()
		offset += int64(len(line) + 1)
		e, payload, ok := nativeRecord(line)
		if first {
			first = false
			if !ok || e.Type != nativeSessionMeta || payload.ID != sessionID {
				return nil, after, errors.New("read Codex skills: rollout session ID mismatch")
			}
		}
		if !ok {
			continue
		}
		fields := object(e.Payload)
		if dir := jsonString(fields[cwdField]); dir != "" && (e.Type == nativeSessionMeta || e.Type == nativeTurnContext) {
			cwd = dir
		}
		if e.Type != responseItem {
			continue
		}
		consumeSkillRecord(e, payload, fields, uses, calls, cwd, offset, after)
	}
	if err := scanner.Err(); err != nil {
		return nil, after, fmt.Errorf("read Codex skills: %w", err)
	}
	if first {
		return nil, after, errors.New("read Codex skills: missing rollout header")
	}
	return trace.SortedSkillUses(uses), offset, nil
}

func skillTag(text, tag string) string {
	_, tail, ok := strings.Cut(text, "<"+tag+">")
	if !ok {
		return ""
	}
	value, _, ok := strings.Cut(tail, "</"+tag+">")
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func resolveSkillPath(path, cwd string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	if !filepath.IsAbs(path) {
		if !filepath.IsAbs(cwd) {
			return ""
		}
		path = filepath.Join(cwd, path)
	}
	return filepath.Clean(path)
}

func skillDefinitionName(text string) string {
	front, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return ""
	}
	meta, _, ok := strings.Cut(front, "\n---")
	if !ok {
		return ""
	}
	var header struct {
		Name string `yaml:"name"`
	}
	if yaml.Unmarshal([]byte(meta), &header) != nil {
		return ""
	}
	return header.Name
}

// SkillRoots classifies the filesystem-backed skill scopes known to Codex.
func (p *Provider) SkillRoots(repoRoot string) skill.Roots {
	home, _ := os.UserHomeDir()
	return skill.Roots{
		Plugin:       []string{filepath.Join(codexHome(), "plugins")},
		Organization: []string{"/etc/codex/skills"},
		User:         []string{filepath.Join(home, ".agents", "skills"), filepath.Join(codexHome(), "skills")},
		Project:      []string{repoRoot},
	}
}

func consumeSkillRecord(e rolloutEntry, payload rolloutPayload, fields jsonObject, uses map[string]*trace.SkillUse, calls map[string]skillCall, cwd string, offset, after int64) {
	record := func(name, path string, byModel bool) {
		if offset <= after || name == "" || filepath.Base(path) != skill.DefinitionFile {
			return
		}
		path = resolveSkillPath(path, cwd)
		if path == "" {
			return
		}
		dir := filepath.Dir(path)
		if uses[dir] == nil {
			uses[dir] = &trace.SkillUse{Name: name, Dir: dir}
		}
		uses[dir].Record(eventTimestamp(e.Timestamp), byModel, false)
	}
	switch payload.Type {
	case messageItem:
		if payload.Role != trace.RoleUser {
			return
		}
		var blocks []struct{ Text string }
		_ = json.Unmarshal(fields[contentField], &blocks)
		for _, b := range blocks {
			text := strings.TrimSpace(b.Text)
			if !strings.HasPrefix(text, "<skill>") || !strings.HasSuffix(text, "</skill>") {
				continue
			}
			name := skillTag(text, "name")
			path := skillTag(text, "path")
			record(name, path, false)
		}
	case functionCall, customToolCall:
		if !isReadTool(payload.Name) {
			return
		}
		input := jsonString(fields["input"])
		callCwd := cwd
		if args := object(json.RawMessage(jsonString(fields["arguments"]))); args != nil {
			input = jsonString(args["cmd"])
			if input == "" {
				input = jsonString(args["command"])
			}
			if dir := jsonString(args["workdir"]); dir != "" {
				callCwd = dir
			}
		}
		calls[payload.CallID] = skillCall{paths: skillReadPaths(input, callCwd), cwd: callCwd}
	case functionCallOutput, customToolCallOutput:
		call := calls[payload.CallID]
		delete(calls, payload.CallID)
		// A catalog or path mention cannot prove a load. Require a tool
		// result containing a skill definition and one identified file.
		if len(call.paths) != 1 {
			return
		}
		for _, text := range outputTexts(fields[outputField]) {
			name := skillDefinitionName(text)
			path := resolveSkillPath(call.paths[0], call.cwd)
			record(name, path, true)
		}
	}
}

func skillReadPaths(input, cwd string) []string {
	literals := codeStringPattern.FindAllString(input, -1)
	inputs := make([]string, 1, len(literals)+1)
	inputs[0] = input
	for _, literal := range literals {
		inputs = append(inputs, jsonString(json.RawMessage(literal)))
	}
	var paths []string
	for _, command := range inputs {
		for _, match := range catPathPattern.FindAllStringSubmatch(command, -1) {
			for _, path := range match[1:] {
				if filepath.Base(path) == skill.DefinitionFile {
					paths = append(paths, resolveSkillPath(path, cwd))
				}
			}
		}
	}
	if len(paths) == 0 {
		for _, path := range skillPathPattern.FindAllString(input, -1) {
			if resolved := resolveSkillPath(path, cwd); resolved != "" {
				paths = append(paths, resolved)
			}
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}
