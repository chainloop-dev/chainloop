package opencode

import (
	"context"
	"encoding/json"
	"os/exec"
	"sort"
	"time"
)

// discoverOpenCodeSession runs `opencode session list --format json`, filters
// sessions whose directory matches repoRoot, and returns the most recently
// updated one. Returns nil, nil when the binary is missing, the command fails,
// or no matching session is found — discovery is best-effort and must never
// block pre-push.
func discoverOpenCodeSession(repoRoot string) (*sessionListEntry, error) {
	if !opencodeBinaryAvailable() {
		return nil, nil
	}

	// Bound the call so a hung opencode binary can never block pre-push.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "opencode", "session", "list", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	var sessions []sessionListEntry
	if err := json.Unmarshal(out, &sessions); err != nil {
		return nil, nil
	}

	// Keep only sessions whose directory matches repoRoot. opencode stores
	// the absolute directory the session was started in; compare verbatim.
	var matching []sessionListEntry
	for _, s := range sessions {
		if s.Directory == repoRoot {
			matching = append(matching, s)
		}
	}

	if len(matching) == 0 {
		return nil, nil
	}

	// Sort by Updated descending — most recent first.
	sort.Slice(matching, func(i, j int) bool {
		return matching[i].Updated > matching[j].Updated
	})

	return &matching[0], nil
}
