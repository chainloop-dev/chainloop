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
	"os"
	"path/filepath"

	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/skill"
)

func modelSkillPath(path, cwd string) bool {
	for _, root := range modelSkillRoots(cwd) {
		if skill.Within(path, root) {
			return true
		}
	}

	return false
}

func modelSkillRoots(cwd string) []string {
	if !filepath.IsAbs(cwd) {
		return nil
	}

	stop := ""
	if _, root, err := tracegit.FindGitDirAndRootFrom(cwd); err == nil {
		stop = root
	}
	roots := []string{
		filepath.Join(cwd, ".pi", "skills"),
		filepath.Join(cwd, ".pi", "git"),
		filepath.Join(cwd, ".pi", "npm", "node_modules"),
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		roots = append(roots, filepath.Join(dir, ".agents", "skills"))
		if dir == stop || filepath.Dir(dir) == dir {
			break
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return roots
	}
	agentDir := piAgentDir(cwd)
	if agentDir != "" {
		roots = append(roots,
			filepath.Join(agentDir, "skills"),
			filepath.Join(agentDir, "git"),
			filepath.Join(agentDir, "npm", "node_modules"),
		)
	}
	return append(roots, filepath.Join(home, ".agents", "skills"))
}

func piAgentDir(base string) string {
	agentDir := os.Getenv("PI_CODING_AGENT_DIR")
	if agentDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		agentDir = filepath.Join(home, ".pi", "agent")
	}

	return skillFilePath(agentDir, base)
}

// SkillRoots follows Pi's user/project and managed package locations. Pi does
// not identify an organization scope; arbitrary external paths stay unknown.
func (p *Provider) SkillRoots(repoRoot string) skill.Roots {
	home, err := os.UserHomeDir()
	if err != nil {
		return skill.Roots{Project: []string{repoRoot}}
	}
	agentDir := piAgentDir(repoRoot)
	return skill.Roots{
		Plugin: []string{
			filepath.Join(agentDir, "git"), filepath.Join(agentDir, "npm", "node_modules"),
			filepath.Join(repoRoot, ".pi", "git"), filepath.Join(repoRoot, ".pi", "npm", "node_modules"),
		},
		User:    []string{filepath.Join(agentDir, "skills"), filepath.Join(home, ".agents", "skills")},
		Project: []string{repoRoot},
	}
}
