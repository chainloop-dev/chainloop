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

// PolicyInputFromFile describes a single --policy-input-from-file[-replace] flag
// value: a policy input name fed from a named column of a CSV or JSON file.
type PolicyInputFromFile struct {
	// Policy optionally scopes the input to a specific policy (its name or ref).
	// Empty means the input is global and applies to every declaring policy.
	Policy string
	// Input is the destination policy input name (e.g. "ignored_paths").
	Input string
	// Column is the file column/field to extract. Defaults to Input.
	Column string
	// File is the source CSV or JSON file path.
	File string
	// Replace reports whether the extracted values replace the contract-declared
	// value for the input (--policy-input-from-file-replace) rather than being
	// appended to it (--policy-input-from-file).
	Replace bool
}

// PolicyInput describes a single --policy-input flag value: a policy input name
// set to a literal value supplied directly on the command line. The value
// always replaces (overrides) any contract-declared value for the input rather
// than being appended, which is what makes a scalar input overridable at run
// time.
type PolicyInput struct {
	// Policy optionally scopes the input to a specific policy (its name or ref).
	// Empty means the input is global and applies to every declaring policy.
	Policy string
	// Input is the destination policy input name (e.g. "min_iterations").
	Input string
	// Value is the literal value to set.
	Value string
}

// scopeDelimiter separates an optional policy scope from the input name on the
// left-hand side of a --policy-input-from-file value. It is ":" (shell-inert in
// bash, sh and zsh). A policy ref can itself contain ":" (in a "://" scheme or
// an "@sha256:" digest) while an input name never can, so the scope is split off
// at the *last* ":" of the left-hand side.
const scopeDelimiter = ":"

// digestScheme is the digest prefix used in versioned policy refs
// (<policy>@sha256:<digest>). Used to detect a versioned scope whose input name
// was omitted, which would otherwise be mistaken for the digest.
const digestScheme = "@sha256"

// parsePolicyInputKey splits the left-hand side of a policy-input flag value
// (everything before "=") into an optional "<policy>:" scope prefix and the
// input name. The optional prefix scopes the input to a single policy (matched
// against its name or ref); without it the input is global. Because a policy ref
// may itself contain ":" (scheme, digest) but an input name never does, the
// scope is taken as everything before the *last* ":". flag names the originating
// CLI flag so errors point at the right one.
func parsePolicyInputKey(lhs, raw, flag string) (policy, input string, err error) {
	if i := strings.LastIndex(lhs, scopeDelimiter); i >= 0 {
		policy = strings.TrimSpace(lhs[:i])
		input = strings.TrimSpace(lhs[i+1:])
		if policy == "" {
			return "", "", fmt.Errorf("invalid %s %q: missing policy scope before %q", flag, raw, scopeDelimiter)
		}
		// A bare "<policy>@sha256:<digest>" (no input) would be mis-split into
		// policy "<policy>@sha256" and input "<digest>"; reject it with guidance.
		// The right-hand side is left unnamed so the message stays accurate for
		// every flag (a file for --policy-input-from-file[-replace], a literal
		// value for --policy-input).
		if strings.HasSuffix(policy, digestScheme) {
			return "", "", fmt.Errorf("invalid %s %q: versioned policy scope is missing an input name; expected an input name after the digest, as in <policy>@sha256:<digest>:<input>", flag, raw)
		}
	} else {
		input = strings.TrimSpace(lhs)
	}

	if input == "" {
		return "", "", fmt.Errorf("invalid %s %q: missing input name", flag, raw)
	}

	return policy, input, nil
}

// ParsePolicyInputFromFile parses a single flag value of the form
// "[<policy>:]<input>=<file>[:<column>]". The optional "<policy>:" prefix scopes
// the input to a single policy (matched against its name or ref); without it the
// input is global. Because a policy ref may itself contain ":" but an input name
// never does, the scope is taken as everything before the *last* ":" on the
// left of "=". The column is optional and defaults to the input name. A column
// is always a single, top-level field/header name — never a path or a nested
// key. The column is the segment after the last ":"; since a column name never
// contains a path separator, a trailing ":<...>" whose ":" belongs to the file
// (a Windows drive letter like C:\data\... or a URL scheme like https://) is not
// mistaken for a column. replace records whether the values replace the contract
// value (--policy-input-from-file-replace) rather than being appended to it
// (--policy-input-from-file), and only affects the flag name shown in errors.
func ParsePolicyInputFromFile(raw string, replace bool) (*PolicyInputFromFile, error) {
	flag := "--policy-input-from-file"
	if replace {
		flag = "--policy-input-from-file-replace"
	}

	lhs, rhs, found := strings.Cut(raw, "=")
	if !found {
		return nil, fmt.Errorf("invalid %s %q: expected [<policy>:]<input>=<file>[:<column>]", flag, raw)
	}

	policy, input, err := parsePolicyInputKey(lhs, raw, flag)
	if err != nil {
		return nil, err
	}

	rhs = strings.TrimSpace(rhs)
	if rhs == "" {
		return nil, fmt.Errorf("invalid %s %q: missing file path", flag, raw)
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
		return nil, fmt.Errorf("invalid %s %q: missing file path", flag, raw)
	}

	return &PolicyInputFromFile{Policy: policy, Input: input, Column: column, File: file, Replace: replace}, nil
}

// ParsePolicyInput parses a single --policy-input flag value of the form
// "[<policy>:]<input>=<value>", where <value> is a literal set directly on the
// command line. The optional "<policy>:" prefix has the same scoping semantics
// as --policy-input-from-file. The value always overrides (replaces) any
// contract-declared value for the input.
func ParsePolicyInput(raw string) (*PolicyInput, error) {
	const flag = "--policy-input"

	lhs, rhs, found := strings.Cut(raw, "=")
	if !found {
		return nil, fmt.Errorf("invalid %s %q: expected [<policy>:]<input>=<value>", flag, raw)
	}

	policy, input, err := parsePolicyInputKey(lhs, raw, flag)
	if err != nil {
		return nil, err
	}

	value := strings.TrimSpace(rhs)
	if value == "" {
		return nil, fmt.Errorf("invalid %s %q: missing value", flag, raw)
	}

	return &PolicyInput{Policy: policy, Input: input, Value: value}, nil
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
