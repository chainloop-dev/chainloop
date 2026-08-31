package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func discoverClaudeSession(repoRoot string) (*claudeSession, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil
	}

	sessionsDir := filepath.Join(homeDir, ".claude", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		// No sessions directory means no sessions
		return nil, nil
	}

	var candidates []claudeSession
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
		if err != nil {
			continue
		}

		var sf claudeSessionFile
		if err := json.Unmarshal(data, &sf); err != nil {
			continue
		}

		if sf.CWD != repoRoot {
			continue
		}

		isActive := isPIDAlive(sf.PID)
		jsonlPath := resolveJSONLPath(homeDir, sf.CWD, sf.SessionID)

		candidates = append(candidates, claudeSession{
			sessionID: sf.SessionID,
			cwd:       sf.CWD,
			pid:       sf.PID,
			isActive:  isActive,
			jsonlPath: jsonlPath,
			startedAt: sf.StartedAt,
		})
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Sort: active sessions first, then by StartedAt descending
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].isActive != candidates[j].isActive {
			return candidates[i].isActive
		}

		return candidates[i].startedAt > candidates[j].startedAt
	})

	best := candidates[0]

	return &best, nil
}

func resolveJSONLPath(homeDir, cwd, sessionID string) string {
	encodedCWD := encodeCWDForClaudePath(cwd)

	return filepath.Join(homeDir, ".claude", "projects", encodedCWD, sessionID+".jsonl")
}

// encodeCWDForClaudePath encodes a directory path the way Claude Code does for project directories.
// Example: /Users/me/projects/myrepo -> -Users-me-projects-myrepo
// Example: /home/user/project/.claude/worktrees/name -> -home-user-project--claude-worktrees-name
func encodeCWDForClaudePath(cwd string) string {
	encoded := strings.ReplaceAll(cwd, string(filepath.Separator), "-")

	return strings.ReplaceAll(encoded, ".", "-")
}
