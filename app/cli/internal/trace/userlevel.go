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

package trace

import (
	"os"
	"path/filepath"
)

// IsUserLevelRoot reports whether root is a user-level directory: the user's
// home directory, or an agent's user-level configuration folder ($CODEX_HOME,
// $COPILOT_HOME, defaulting to ~/.codex and ~/.copilot).
//
// Trace installs agent hooks as files relative to the repository root, so a
// root that is the home directory would write the agent's user-level
// configuration (for example ~/.claude/settings.json) and the hooks would
// load in every session of that agent, in any project, instead of only in
// this repository. Callers should refuse such a root before installing
// anything. Symlinks are resolved on both sides, so a symlinked home
// directory is still caught.
func IsUserLevelRoot(root string) (bool, error) {
	resolvedRoot, err := resolveDir(root)
	if err != nil {
		return false, err
	}

	for _, candidate := range userLevelDirs() {
		resolved, err := resolveDir(candidate)
		if err != nil {
			continue
		}
		if resolvedRoot == resolved {
			return true, nil
		}
	}

	return false, nil
}

// resolveDir returns the absolute path of dir with symlinks evaluated. When
// the path does not exist yet, EvalSymlinks fails and the cleaned absolute
// path is used instead, so callers can still compare it.
func resolveDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

// userLevelDirs lists the directories whose contents an agent loads for every
// session: the user's home directory first, then the agent-specific home
// folders where they apply.
func userLevelDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}

	dirs := []string{home}
	if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
		dirs = append(dirs, codexHome)
	} else {
		dirs = append(dirs, filepath.Join(home, ".codex"))
	}
	if copilotHome := os.Getenv("COPILOT_HOME"); copilotHome != "" {
		dirs = append(dirs, copilotHome)
	} else {
		dirs = append(dirs, filepath.Join(home, ".copilot"))
	}
	return dirs
}
