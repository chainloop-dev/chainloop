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

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// Fingerprint returns the content key of one line. Whitespace is dropped
// before hashing, so a line that a change only re-indented keeps its key, and
// with it its owner (D-008).
func Fingerprint(line string) string {
	stripped := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}

		return r
	}, line)

	sum := sha256.Sum256([]byte(stripped))

	// 8 bytes keep the ledger small. A collision only matters between lines
	// of the same file, where it is negligible.
	return hex.EncodeToString(sum[:8])
}

// FingerprintLines returns the fingerprint of each line of content, in order.
func FingerprintLines(content []byte) []string {
	lines := splitLines(string(content))
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, Fingerprint(l))
	}

	return out
}

// splitLines splits text into lines without their line ends. A final line
// with no newline is a line too, and empty text has no lines.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}

	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\n")
	}

	return lines
}
