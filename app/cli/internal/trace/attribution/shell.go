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
