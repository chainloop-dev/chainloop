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

package attribution

import "sort"

// ChangedPaths diffs two working-tree signatures (repo-relative path → content
// hash) captured before and after a shell command, returning the paths the
// command created or modified (changed) and the paths it removed (deleted).
// Both slices are sorted for deterministic recording. Used to attribute
// files touched by agent-run shell commands, which never fire the file-writing
// tool hooks.
func ChangedPaths(before, after map[string]string) (changed, deleted []string) {
	for path, hash := range after {
		if prev, ok := before[path]; !ok || prev != hash {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			deleted = append(deleted, path)
		}
	}

	sort.Strings(changed)
	sort.Strings(deleted)

	return changed, deleted
}
