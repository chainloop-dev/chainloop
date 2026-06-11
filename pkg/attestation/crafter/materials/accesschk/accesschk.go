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

// Package accesschk parses the text output of the Sysinternals AccessChk tool
// (https://learn.microsoft.com/en-us/sysinternals/downloads/accesschk) into a
// structured representation. AccessChk has no machine-readable output mode, so
// the parser is intentionally tolerant: anything it cannot recognize is
// preserved verbatim and the full original text is always retained in Raw, so a
// policy can fall back to string matching regardless of the output mode used.
package accesschk

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ToolName is the canonical tool name recorded for AccessChk materials.
const ToolName = "AccessChk"

// versionRe extracts the AccessChk version from its banner, e.g. "Accesschk v6.15".
var versionRe = regexp.MustCompile(`(?i)accesschk v([0-9][0-9.]*)`)

// accessEntryRe matches a per-principal access line such as "  RW BUILTIN\Administrators".
// The access token (R, W or RW) must be followed by whitespace, which prevents
// right names like "WRITE_DAC" or "READ_CONTROL" from being mistaken for entries.
var accessEntryRe = regexp.MustCompile(`^(RW|R|W)\s+(\S.*)$`)

// aceRe matches a numbered ACE line emitted under -l, in both the DACL form
// "[0] ACCESS_ALLOWED_ACE_TYPE: NT AUTHORITY\SYSTEM" and the SACL form
// "[0] : Everyone" (where the ACE type is empty).
var aceRe = regexp.MustCompile(`^\[(\d+)\]\s*(.*?):\s*(.*)$`)

// Tool holds the tool identity parsed from the AccessChk banner.
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// AccessEntry is a single principal and the access it was granted on an object,
// as reported by the compact default (R/W) output mode.
type AccessEntry struct {
	Access    string   `json:"access"`
	Principal string   `json:"principal"`
	Rights    []string `json:"rights"`
}

// ACE is a single access control entry from a security descriptor reported by
// the -l output mode (DACL or SACL).
type ACE struct {
	Index     int      `json:"index"`
	AceType   string   `json:"ace_type,omitempty"`
	Principal string   `json:"principal"`
	AceFlags  []string `json:"ace_flags"`
	Rights    []string `json:"rights"`
}

// Object is a single securable object reported by AccessChk.
//
// AccessEntries is populated by the compact default mode; DescriptorFlags,
// Owner, DACL and SACL are populated by the -l (full security descriptor) mode.
// RawLines always holds every indented line verbatim regardless of mode.
type Object struct {
	Name            string        `json:"name"`
	DescriptorFlags []string      `json:"descriptor_flags,omitempty"`
	Owner           string        `json:"owner,omitempty"`
	DACL            []ACE         `json:"dacl,omitempty"`
	SACL            []ACE         `json:"sacl,omitempty"`
	AccessEntries   []AccessEntry `json:"access_entries"`
	RawLines        []string      `json:"raw_lines"`
}

// Report is the structured projection of an AccessChk run.
type Report struct {
	Tool    Tool     `json:"tool"`
	Objects []Object `json:"objects"`
	Raw     string   `json:"raw"`
}

// Parse converts AccessChk text output into a Report. It only returns an error
// when the input is not valid UTF-8 text; well-formed text always parses, with
// any unrecognized content preserved in the per-object RawLines and the
// top-level Raw field.
func Parse(data []byte) (*Report, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("input is not valid UTF-8 text")
	}

	raw := string(data)
	report := &Report{
		Tool:    Tool{Name: ToolName},
		Objects: []Object{},
		Raw:     raw,
	}

	if m := versionRe.FindStringSubmatch(raw); m != nil {
		report.Tool.Version = m[1]
	}

	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	var current *Object
	var entryIndent int

	// State for the -l (full security descriptor) mode.
	const (
		sectNone = iota
		sectDescriptorFlags
		sectDACL
		sectSACL
	)
	section := sectNone
	var currentACE *ACE

	for _, line := range strings.Split(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isBannerLine(trimmed) {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		// A line at column zero starts a new object.
		if indent == 0 {
			report.Objects = append(report.Objects, Object{
				Name:          trimmed,
				AccessEntries: []AccessEntry{},
				RawLines:      []string{},
			})
			current = &report.Objects[len(report.Objects)-1]
			entryIndent = -1
			section = sectNone
			currentACE = nil
			continue
		}

		// Indented content before any object header is dropped.
		if current == nil {
			continue
		}

		current.RawLines = append(current.RawLines, line)

		// Compact default (R/W) output mode.
		if m := accessEntryRe.FindStringSubmatch(trimmed); m != nil {
			current.AccessEntries = append(current.AccessEntries, AccessEntry{
				Access:    m[1],
				Principal: m[2],
				Rights:    []string{},
			})
			entryIndent = indent
			currentACE = nil
			continue
		}

		// -l (full security descriptor) section headers.
		switch {
		case strings.HasPrefix(trimmed, "DESCRIPTOR FLAGS"):
			section = sectDescriptorFlags
			currentACE = nil
			continue
		case strings.HasPrefix(trimmed, "OWNER:"):
			current.Owner = strings.TrimSpace(strings.TrimPrefix(trimmed, "OWNER:"))
			currentACE = nil
			continue
		case strings.HasPrefix(trimmed, "DACL"):
			section = sectDACL
			currentACE = nil
			continue
		case strings.HasPrefix(trimmed, "SACL"):
			section = sectSACL
			currentACE = nil
			continue
		}

		// -l numbered ACE lines (DACL by default, SACL once inside a SACL block).
		if m := aceRe.FindStringSubmatch(trimmed); m != nil {
			ace := ACE{
				Index:     atoi(m[1]),
				AceType:   strings.TrimSpace(m[2]),
				Principal: strings.TrimSpace(m[3]),
				AceFlags:  []string{},
				Rights:    []string{},
			}
			if section == sectSACL {
				current.SACL = append(current.SACL, ace)
				currentACE = &current.SACL[len(current.SACL)-1]
			} else {
				section = sectDACL
				current.DACL = append(current.DACL, ace)
				currentACE = &current.DACL[len(current.DACL)-1]
			}
			continue
		}

		// Detail lines: bracketed tokens are flags, bare tokens are rights.
		isFlag := strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")
		token := strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")

		if currentACE != nil {
			if isFlag {
				currentACE.AceFlags = append(currentACE.AceFlags, token)
			} else {
				currentACE.Rights = append(currentACE.Rights, trimmed)
			}
			continue
		}

		if section == sectDescriptorFlags {
			current.DescriptorFlags = append(current.DescriptorFlags, token)
			continue
		}

		// A line indented deeper than the compact access entry it follows is a
		// specific right (only emitted under -v); attach it to the entry.
		if entryIndent >= 0 && indent > entryIndent && len(current.AccessEntries) > 0 {
			last := &current.AccessEntries[len(current.AccessEntries)-1]
			last.Rights = append(last.Rights, trimmed)
		}
	}

	return report, nil
}

// atoi parses a non-negative integer, returning 0 on failure. ACE indexes are
// always well-formed in AccessChk output, so this keeps the caller simple.
func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// LooksLikeAccessChk reports whether the parsed report resembles genuine
// AccessChk output. It is deliberately lenient: a recognizable banner, at least
// one parsed access entry, or an SDDL/descriptor marker is enough.
func (r *Report) LooksLikeAccessChk() bool {
	if r.Tool.Version != "" {
		return true
	}
	for _, o := range r.Objects {
		if len(o.AccessEntries) > 0 || len(o.DACL) > 0 || len(o.SACL) > 0 ||
			o.Owner != "" || len(o.DescriptorFlags) > 0 {
			return true
		}
	}
	if strings.Contains(r.Raw, "DESCRIPTOR FLAGS") || strings.Contains(r.Raw, "ACCESS_ALLOWED") {
		return true
	}
	return false
}

// isBannerLine reports whether a trimmed line belongs to the AccessChk startup
// banner/copyright, which must not be treated as an object or access entry.
func isBannerLine(trimmed string) bool {
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(lower, "accesschk v"):
		return true
	case strings.HasPrefix(lower, "copyright"):
		return true
	case strings.Contains(lower, "sysinternals - www.sysinternals.com"):
		return true
	default:
		return false
	}
}
