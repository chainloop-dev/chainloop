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

// Package radamsa parses radamsa's -M metadata log into structured records.
package radamsa

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Parse reads a radamsa -M metadata log and returns one record per non-blank
// line. Each record is a map of key -> value, where quoted values are unquoted
// strings, integer-looking bare values are int64, and other bare tokens are
// strings. It errors if no parseable record is found.
func Parse(r io.Reader) ([]map[string]any, error) {
	scanner := bufio.NewScanner(r)
	// radamsa lines can be long (many fields); raise the buffer ceiling.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	records := make([]map[string]any, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		rec, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("no radamsa -M records found")
	}
	return records, nil
}

func parseLine(line string) (map[string]any, error) {
	rec := make(map[string]any)
	for _, pair := range splitTopLevel(line) {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, rawVal, found := strings.Cut(pair, ": ")
		if !found {
			// allow a trailing "key:" with no value, but a token with no colon
			// at all is not a valid meta-log pair.
			trimmed, ok := strings.CutSuffix(pair, ":")
			if !ok {
				return nil, fmt.Errorf("invalid radamsa -M pair: %q", pair)
			}
			key = trimmed
		}
		rec[strings.TrimSpace(key)] = parseValue(strings.TrimSpace(rawVal))
	}
	if len(rec) == 0 {
		return nil, fmt.Errorf("invalid radamsa -M line: %q", line)
	}
	return rec, nil
}

// splitTopLevel splits on ", " but not inside double-quoted spans.
func splitTopLevel(line string) []string {
	var parts []string
	var b strings.Builder
	inQuote := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\\' && i+1 < len(line):
			// Preserve an escaped character verbatim (e.g. \" inside a quoted
			// value) so it neither flips the quote state nor acts as a delimiter;
			// strconv.Unquote unescapes it later in parseValue.
			b.WriteByte(c)
			b.WriteByte(line[i+1])
			i++
		case c == '"':
			inQuote = !inQuote
			b.WriteByte(c)
		case !inQuote && c == ',' && i+1 < len(line) && line[i+1] == ' ':
			parts = append(parts, b.String())
			b.Reset()
			i++ // skip the space
		default:
			b.WriteByte(c)
		}
	}
	parts = append(parts, b.String())
	return parts
}

func parseValue(v string) any {
	if len(v) >= 2 && strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) {
		unq, err := strconv.Unquote(v)
		if err != nil {
			// fall back to trimming the surrounding quotes on non-Go-escaped text
			return strings.Trim(v, `"`)
		}
		return unq
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		return n
	}
	return v
}
