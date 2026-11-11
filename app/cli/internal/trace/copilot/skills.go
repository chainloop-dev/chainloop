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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/skill"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

var _ trace.SkillTracker = (*Provider)(nil)

// mainLogKey is the key of the event log in the skill scan offsets. Copilot
// writes one log for each session, subagents included.
const mainLogKey = "main"

// SessionSkills reads the skills from the private copy of the event log. Only
// a skill.invoked event counts as a use: Copilot writes it when it loads a
// skill into the session, not when it lists the skills (spec issue-3800
// R-008).
func (p *Provider) SessionSkills(opts *trace.ParseOpts) ([]trace.SkillUse, []string, error) {
	if opts == nil || !state.ValidSessionID(opts.SessionID) {
		return nil, nil, errors.New("read Copilot skills: invalid session")
	}
	path, err := state.FindRawSessionFile(opts.SessionDir, opts.SessionID)
	if err != nil {
		return nil, nil, err
	}
	uses, _, err := scanSkills(path, opts.SessionID, 0, false)
	return uses, nil, err
}

// NewSkillLoads returns the folders of the skills that the session loaded
// since the last call, from the live event log. It reads only a log inside
// the Copilot session-state folder, and never fails.
func (p *Provider) NewSkillLoads(store *state.Store, input *trace.HookInput) []string {
	if store == nil || input == nil || !state.ValidSessionID(input.SessionID) {
		return nil
	}
	root := sessionStateRoot()
	if rec, err := store.LoadSessionRecord(input.SessionID); err == nil && rec != nil && rec.CopilotStorageRoot != "" {
		root = rec.CopilotStorageRoot
	}
	if root == "" {
		return nil
	}
	path := input.TranscriptPath
	if path == "" {
		path = filepath.Join(root, input.SessionID, eventsFile)
	}
	if !filepath.IsAbs(path) {
		return nil
	}
	resolved, err := resolveInsideRoot(path, root)
	if err != nil {
		return nil
	}

	offsets, err := store.SkillScanOffsets(input.SessionID)
	if err != nil || offsets == nil {
		offsets = map[string]int64{}
	}
	uses, next, err := scanSkills(resolved, input.SessionID, offsets[mainLogKey], true)
	if err != nil {
		return nil
	}
	offsets[mainLogKey] = next
	_ = store.SetSkillScanOffsets(input.SessionID, offsets)

	dirs := make([]string, 0, len(uses))
	for _, use := range uses {
		dirs = append(dirs, use.Dir)
	}
	return dirs
}

// skillInvokedData holds the fields of a skill.invoked event that Trace reads.
type skillInvokedData struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Trigger string `json:"trigger"`
}

// scanSkills returns the skill uses of the log that end after the byte offset
// after, and the offset of the end of the last complete line. In live mode a
// last line with no newline is not read: Copilot is still writing it.
func scanSkills(path, sessionID string, after int64, live bool) ([]trace.SkillUse, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, after, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, after, errors.New("read Copilot skills: not a regular event log")
	}
	if info.Size() < after {
		// The log is shorter than at the last scan: it is a new log.
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
	var offset int64
	first := true
	for scanner.Scan() {
		line := scanner.Bytes()
		offset += int64(len(line) + 1)
		var e logEvent
		ok := json.Unmarshal(line, &e) == nil
		if first {
			first = false
			var data eventData
			if !ok || e.Type != eventSessionStartRecord || json.Unmarshal(e.Data, &data) != nil || data.SessionID != sessionID {
				return nil, after, errors.New("read Copilot skills: session ID mismatch")
			}
			continue
		}
		if !ok || e.Type != eventSkillInvoked || offset <= after {
			continue
		}
		var data skillInvokedData
		if json.Unmarshal(e.Data, &data) != nil || data.Name == "" || !filepath.IsAbs(data.Path) || filepath.Base(data.Path) != skill.DefinitionFile {
			continue
		}
		dir := filepath.Dir(filepath.Clean(data.Path))
		if uses[dir] == nil {
			uses[dir] = &trace.SkillUse{Name: data.Name, Dir: dir}
		}
		// The user starts a skill with a slash command. Copilot marks a skill
		// that the model loaded as "agent-invoked".
		byUser := strings.HasPrefix(data.Trigger, "user")
		uses[dir].Record(eventTimestamp(e.Timestamp), !byUser, e.AgentID != "")
	}
	if err := scanner.Err(); err != nil {
		return nil, after, fmt.Errorf("read Copilot skills: %w", err)
	}
	if first {
		return nil, after, errors.New("read Copilot skills: missing session.start event")
	}
	return trace.SortedSkillUses(uses), offset, nil
}

// eventTimestamp returns the time of an event in RFC3339 with whole seconds,
// in UTC, or "" when it cannot be read.
func eventTimestamp(timestamp string) string {
	at, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

// SkillRoots returns the skill folders that Copilot reads, which tell where a
// skill came from. Copilot reads project skills in .github/skills,
// .agents/skills and .claude/skills, all under the repository.
func (p *Provider) SkillRoots(repoRoot string) skill.Roots {
	userHome, _ := os.UserHomeDir()
	home := copilotHomeDir()
	roots := skill.Roots{Project: []string{repoRoot}}
	if home != "" {
		roots.Plugin = []string{filepath.Join(home, "installed-plugins")}
		roots.User = append(roots.User, filepath.Join(home, "skills"))
	}
	if userHome != "" {
		roots.User = append(roots.User, filepath.Join(userHome, ".agents", "skills"))
	}
	return roots
}
