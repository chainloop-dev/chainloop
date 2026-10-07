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

package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/skill"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

// Compile-time check that Provider records skills.
var _ trace.SkillTracker = (*Provider)(nil)

// skillTool is the native OpenCode tool that loads a skill. Its one input is
// the name of the skill.
const skillTool = "skill"

// NewSkillLoads returns the skill folder that the plugin put in the payload
// of the hook after a skill tool call. OpenCode tells the folder in the tool
// result, so no transcript is read.
func (p *Provider) NewSkillLoads(_ *state.Store, input *trace.HookInput) []string {
	if input.ToolName != skillTool || input.SkillDir == "" {
		return nil
	}

	return []string{input.SkillDir}
}

// SessionSkills returns the skills that the session used, from the copied
// export. A model start is a completed call to the skill tool. OpenCode has
// no slash command that starts a skill, so the user count stays at zero. A
// subagent runs in a child session, which has its own export, so the count of
// subagent uses stays at zero too.
func (p *Provider) SessionSkills(opts *trace.ParseOpts) ([]trace.SkillUse, []string, error) {
	path, err := state.FindRawSessionFile(opts.SessionDir, opts.SessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("locate opencode export: %w", err)
	}

	export, err := readExport(path)
	if err != nil {
		return nil, nil, err
	}

	return exportSkills(export), nil, nil
}

// exportSkills counts the completed skill tool calls of an export.
func exportSkills(export *exportData) []trace.SkillUse {
	uses := make(map[string]*trace.SkillUse)
	for _, msg := range export.Messages {
		at := msgTimeRFC3339(msg.Info.Time)

		for _, part := range msg.Parts {
			if part.Type != partTool || part.Tool != skillTool || part.State == nil || part.State.Status != toolStatusCompleted {
				continue
			}

			var input struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(part.State.Input, &input) != nil || strings.TrimSpace(input.Name) == "" {
				continue
			}
			name := strings.TrimSpace(input.Name)

			use, ok := uses[name]
			if !ok {
				use = &trace.SkillUse{Name: name}
				uses[name] = use
			}
			if use.Dir == "" {
				use.Dir = toolSkillDir(part.State)
			}
			use.Record(at, true, false)
		}
	}

	return trace.SortedSkillUses(uses)
}

// toolSkillDir returns the skill folder of a skill tool call: from the tool
// metadata, or else from the base directory line of the tool output.
func toolSkillDir(st *toolState) string {
	if st.Metadata != nil && filepath.IsAbs(st.Metadata.Dir) {
		return filepath.Clean(st.Metadata.Dir)
	}

	var output string
	if json.Unmarshal(st.Output, &output) != nil {
		return ""
	}

	dir, _ := skill.BaseDir(output)

	return dir
}

// SkillRoots returns the OpenCode folders that tell where a skill came from.
// OpenCode reads project skills from .opencode/skills, .claude/skills and
// .agents/skills in the repository, and user skills from the same folders in
// the user's configuration.
func (p *Provider) SkillRoots(repoRoot string) skill.Roots {
	roots := skill.Roots{Project: []string{repoRoot}}

	home, err := os.UserHomeDir()
	if err != nil {
		return roots
	}

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	roots.User = []string{
		filepath.Join(configHome, "opencode", "skills"),
		filepath.Join(home, ".opencode", "skills"),
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".agents", "skills"),
	}

	return roots
}
