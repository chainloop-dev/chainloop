package state

import (
	"sort"
	"strings"
)

// TrailerKey is the git-style trailer the commit-msg hook writes when AI
// sessions are attributed to staged files. The key is matched case-sensitively
// per git trailer convention.
const TrailerKey = "Chainloop-Trace-Sessions"

// ParseSessionIDsFromTrailer scans a commit message for `Chainloop-Trace-Sessions:`
// trailer lines and returns the sorted, deduped list of session IDs declared in
// the message. Lines must start with the trailer key (no leading whitespace) to
// match git's trailer parsing rules. Empty values and blank entries between
// commas are ignored.
func ParseSessionIDsFromTrailer(message string) []string {
	if message == "" {
		return nil
	}

	seen := make(map[string]struct{})
	for line := range strings.SplitSeq(message, "\n") {
		values, ok := strings.CutPrefix(line, TrailerKey+":")
		if !ok {
			continue
		}

		for v := range strings.SplitSeq(values, ",") {
			v = strings.TrimSpace(v)
			// Trailers come from commit messages, so an ID that cannot name a
			// file is either corrupt or hostile; either way it is not ours.
			if !ValidSessionID(v) {
				continue
			}
			seen[v] = struct{}{}
		}
	}

	if len(seen) == 0 {
		return nil
	}

	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)

	return out
}
