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

package action

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chainloop-dev/chainloop/pkg/tabular"
)

// PolicyInputFromFile describes a single --policy-input-from-file flag value: a
// policy input name fed from a named column of a CSV or JSON file.
type PolicyInputFromFile struct {
	// Input is the destination policy input name (e.g. "ignored_paths").
	Input string
	// Column is the file column/field to extract. Defaults to Input.
	Column string
	// File is the source CSV or JSON file path.
	File string
}

// ParsePolicyInputFromFile parses a single flag value of the form
// "<input>=<file>[:<column>]". The column is optional and defaults to the input
// name. A column is always a single, top-level field/header name — never a path
// or a nested key. The column is the segment after the last ":"; since a column
// name never contains a path separator, a trailing ":<...>" whose ":" belongs to
// the file (a Windows drive letter like C:\data\... or a URL scheme like
// https://) is not mistaken for a column.
func ParsePolicyInputFromFile(raw string) (*PolicyInputFromFile, error) {
	input, rhs, found := strings.Cut(raw, "=")
	if !found {
		return nil, fmt.Errorf("invalid --policy-input-from-file %q: expected <input>=<file>[:<column>]", raw)
	}

	input = strings.TrimSpace(input)
	rhs = strings.TrimSpace(rhs)
	if input == "" {
		return nil, fmt.Errorf("invalid --policy-input-from-file %q: missing input name", raw)
	}
	if rhs == "" {
		return nil, fmt.Errorf("invalid --policy-input-from-file %q: missing file path", raw)
	}

	// Default the column to the input name; override it only when a ":<column>"
	// suffix is present and unambiguously a column (no path separator).
	file := rhs
	column := input
	if i := strings.LastIndex(rhs, ":"); i >= 0 {
		if candidate := strings.TrimSpace(rhs[i+1:]); candidate != "" && !strings.ContainsAny(candidate, `/\`) {
			file = strings.TrimSpace(rhs[:i])
			column = candidate
		}
	}

	if file == "" {
		return nil, fmt.Errorf("invalid --policy-input-from-file %q: missing file path", raw)
	}

	return &PolicyInputFromFile{Input: input, Column: column, File: file}, nil
}

// ExtractColumnValues reads the given CSV or JSON file and returns the values of
// the named column/field. Format is detected by extension, with a content-sniff
// fallback. Empty and whitespace-only values are dropped. CSV parsing reuses the
// tabular parser (BOM decoding, comma/tab auto-detection, case-insensitive
// header match).
func ExtractColumnValues(path, column string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading policy input file: %w", err)
	}

	// Strip a leading UTF-8 BOM (common on Windows-authored files) so both
	// format detection and JSON parsing see clean bytes. The CSV path strips
	// it again inside tabular.Parse, which is harmless.
	content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))

	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return extractJSONColumn(content, column)
	case ".csv", ".tsv", ".txt":
		return extractCSVColumn(content, column)
	default:
		// Content sniff: a leading "[" or "{" means JSON, otherwise treat as CSV.
		if t := bytes.TrimSpace(content); len(t) > 0 && (t[0] == '[' || t[0] == '{') {
			return extractJSONColumn(content, column)
		}
		return extractCSVColumn(content, column)
	}
}

func extractCSVColumn(content []byte, column string) ([]string, error) {
	table, err := tabular.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parsing CSV policy input file: %w", err)
	}

	values, ok := table.Column(column)
	if !ok {
		return nil, fmt.Errorf("column %q not found in CSV header %v", column, table.Header)
	}

	return values, nil
}

// extractJSONColumn extracts column values from one of three accepted shapes:
// a bare array of strings, an array of string-valued objects (the column field
// of each), or an object mapping the column to an array of strings. The column
// is matched only against top-level keys; nested paths are not interpreted.
func extractJSONColumn(content []byte, column string) ([]string, error) {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return nil, errors.New("empty JSON policy input file")
	}

	switch trimmed[0] {
	case '[':
		// Bare array of strings.
		var strs []string
		if err := json.Unmarshal(trimmed, &strs); err == nil {
			return filterNonEmpty(strs), nil
		}

		// Array of string-valued objects: pull the column field from each.
		var objs []map[string]string
		if err := json.Unmarshal(trimmed, &objs); err != nil {
			return nil, fmt.Errorf("parsing JSON array (expected an array of strings or of string-valued objects): %w", err)
		}
		values := make([]string, 0, len(objs))
		for _, obj := range objs {
			if v, ok := matchKey(obj, column); ok {
				values = append(values, v)
			}
		}
		return filterNonEmpty(values), nil
	case '{':
		// Object mapping the column to an array of strings. The values are
		// decoded into a typed []string; sibling keys are left as raw messages
		// so fields of other types don't break the parse.
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return nil, fmt.Errorf("parsing JSON object: %w", err)
		}
		raw, ok := matchKey(obj, column)
		if !ok {
			return nil, fmt.Errorf("key %q not found in JSON object", column)
		}
		var strs []string
		if err := json.Unmarshal(raw, &strs); err != nil {
			return nil, fmt.Errorf("value of %q is not an array of strings: %w", column, err)
		}
		return filterNonEmpty(strs), nil
	default:
		return nil, errors.New("JSON policy input file must be an array or object")
	}
}

// matchKey returns the value whose key matches column case-insensitively
// (trimming surrounding whitespace).
func matchKey[T any](m map[string]T, column string) (T, bool) {
	for k, v := range m {
		if strings.EqualFold(strings.TrimSpace(k), strings.TrimSpace(column)) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

func filterNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
