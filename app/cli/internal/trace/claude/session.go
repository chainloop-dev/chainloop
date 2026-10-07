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
	"path/filepath"
	"strings"
)

// encodeCWDForClaudePath encodes a directory path the way Claude Code does for project directories.
// Example: /Users/me/projects/myrepo -> -Users-me-projects-myrepo
// Example: /home/user/project/.claude/worktrees/name -> -home-user-project--claude-worktrees-name
func encodeCWDForClaudePath(cwd string) string {
	encoded := strings.ReplaceAll(cwd, string(filepath.Separator), "-")

	return strings.ReplaceAll(encoded, ".", "-")
}
