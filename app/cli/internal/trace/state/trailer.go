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
