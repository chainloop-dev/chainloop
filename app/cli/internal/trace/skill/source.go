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

package skill

import (
	"path/filepath"
	"strings"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// Roots are the folders that tell where a skill came from. Each agent has its
// own layout, so each provider gives its own.
type Roots struct {
	// Plugin holds the folders where the agent installs plugins.
	Plugin []string
	// Organization holds the folders of the managed configuration, which an
	// administrator installs for all users.
	Organization []string
	// User holds the skill folders of the user's agent configuration.
	User []string
	// Project holds the repository and the session folder.
	Project []string
}

// Source returns where a skill came from, as one of the
// aicodingsession.SkillSource* constants. A plugin skill has a name with a
// plugin prefix (plugin:skill) or a folder in a plugin folder. Otherwise the
// first root list that holds the folder gives the source, in the order of
// the Roots fields. A folder that no root holds, or no folder, gives unknown.
func Source(name, dir string, roots Roots) string {
	if strings.Contains(name, ":") {
		return aicodingsession.SkillSourcePlugin
	}

	if dir == "" {
		return aicodingsession.SkillSourceUnknown
	}

	for _, r := range []struct {
		roots  []string
		source string
	}{
		{roots.Plugin, aicodingsession.SkillSourcePlugin},
		{roots.Organization, aicodingsession.SkillSourceOrganization},
		{roots.User, aicodingsession.SkillSourceUser},
		{roots.Project, aicodingsession.SkillSourceProject},
	} {
		for _, root := range r.roots {
			if root != "" && withinResolved(dir, root) {
				return r.source
			}
		}
	}

	return aicodingsession.SkillSourceUnknown
}

// withinResolved compares the paths as given, and again with their links
// resolved: on macOS /tmp is a link to /private/tmp, and an agent can report
// either.
func withinResolved(path, root string) bool {
	if Within(path, root) {
		return true
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}

	return Within(resolvedPath, resolvedRoot)
}
