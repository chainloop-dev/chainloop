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
// preserved verbatim and, for inputs below RawRetentionLimit, the full original
// text is retained in Raw so a policy can fall back to string matching
// regardless of the output mode used.
//
// The parser streams the input line by line rather than building a normalized
// full-text copy, and it size-gates the verbatim fallback fields (Raw and the
// per-object RawLines). Both measures keep peak memory bounded: a
// several-hundred-MB material would otherwise pin multiple copies of itself in
// memory and double the JSON document handed to the policy engine, which has
// OOM-killed CI runners during client-side policy evaluation.
package accesschk

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ToolName is the canonical tool name recorded for AccessChk materials.
const ToolName = "AccessChk"

// RawRetentionLimit is the maximum input size (in bytes) for which Parse retains
// the verbatim fallback fields Raw and RawLines. Above it these fields are
// omitted: they are not part of the attestation (only the original file's digest
// is attested) and no policy reads them, so trimming them for oversized inputs
// does not change the recorded evidence or any current evaluation — it only
// prevents the transient JSON projection handed to the policy engine from
// ballooning to multiples of the original file size.
const RawRetentionLimit = 10 * 1024 * 1024 // 10 MiB

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
//
// Raw holds the full original text for inputs below RawRetentionLimit and is
// empty otherwise; descriptorMarker records whether an SDDL/descriptor marker
// was seen during parsing so LooksLikeAccessChk stays reliable even when Raw is
// omitted for oversized inputs.
type Report struct {
	Tool    Tool     `json:"tool"`
	Objects []Object `json:"objects"`
	Raw     string   `json:"raw"`

	descriptorMarker bool
}

// Parse converts AccessChk text output into a Report. It only returns an error
// when the input is not valid UTF-8 text; well-formed text always parses, with
// any unrecognized content preserved in the per-object RawLines and the
// top-level Raw field for inputs below RawRetentionLimit (see the package doc).
func Parse(data []byte) (*Report, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("input is not valid UTF-8 text")
	}

	// Retain the verbatim fallback fields only for inputs small enough that the
	// resulting JSON projection stays bounded; see RawRetentionLimit.
	retainRaw := len(data) <= RawRetentionLimit

	report := &Report{
		Tool:    Tool{Name: ToolName},
		Objects: []Object{},
	}
	if retainRaw {
		report.Raw = string(data)
	}

	var current *Object
	var entryIndent int
	section := sectNone
	var currentACE *ACE

	// Stream line by line instead of normalizing and splitting the whole input
	// at once. bufio.Reader.ReadString grows to fit arbitrarily long lines and
	// returns freshly allocated strings, so stored substrings do not pin the
	// entire input in memory.
	reader := bufio.NewReader(bytes.NewReader(data))
	for {
		raw, readErr := reader.ReadString('\n')
		line := strings.TrimSuffix(raw, "\n")
		line = strings.TrimSuffix(line, "\r")

		if line != "" || readErr == nil {
			processLine(report, line, &current, &entryIndent, &section, &currentACE, retainRaw)
		}

		if readErr != nil {
			if readErr != io.EOF {
				return nil, fmt.Errorf("reading accesschk output: %w", readErr)
			}
			break
		}
	}

	return report, nil
}

// sect* constants for the -l (full security descriptor) mode section state.
const (
	sectNone = iota
	sectDescriptorFlags
	sectDACL
	sectSACL
)

// processLine folds a single (newline-stripped) line into the report, advancing
// the parser's cursor into the current object, access entry and descriptor
// section. It is the per-line body of Parse's streaming loop.
func processLine(report *Report, line string, current **Object, entryIndent *int, section *int, currentACE **ACE, retainRaw bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || isBannerLine(trimmed) {
		if report.Tool.Version == "" {
			if m := versionRe.FindStringSubmatch(trimmed); m != nil {
				report.Tool.Version = m[1]
			}
		}
		return
	}

	// Track SDDL/descriptor markers so LooksLikeAccessChk works without Raw.
	if !report.descriptorMarker &&
		(strings.Contains(trimmed, "DESCRIPTOR FLAGS") || strings.Contains(trimmed, "ACCESS_ALLOWED")) {
		report.descriptorMarker = true
	}

	indent := len(line) - len(strings.TrimLeft(line, " \t"))

	// A line at column zero starts a new object.
	if indent == 0 {
		obj := Object{Name: trimmed, AccessEntries: []AccessEntry{}}
		if retainRaw {
			obj.RawLines = []string{}
		}
		report.Objects = append(report.Objects, obj)
		*current = &report.Objects[len(report.Objects)-1]
		*entryIndent = -1
		*section = sectNone
		*currentACE = nil
		return
	}

	// Indented content before any object header is dropped.
	if *current == nil {
		return
	}

	cur := *current
	if retainRaw {
		cur.RawLines = append(cur.RawLines, line)
	}

	// Compact default (R/W) output mode.
	if m := accessEntryRe.FindStringSubmatch(trimmed); m != nil {
		cur.AccessEntries = append(cur.AccessEntries, AccessEntry{
			Access:    m[1],
			Principal: m[2],
			Rights:    []string{},
		})
		*entryIndent = indent
		*currentACE = nil
		return
	}

	// -l (full security descriptor) section headers.
	switch {
	case strings.HasPrefix(trimmed, "DESCRIPTOR FLAGS"):
		*section = sectDescriptorFlags
		*currentACE = nil
		return
	case strings.HasPrefix(trimmed, "OWNER:"):
		cur.Owner = strings.TrimSpace(strings.TrimPrefix(trimmed, "OWNER:"))
		*currentACE = nil
		return
	case strings.HasPrefix(trimmed, "DACL"):
		*section = sectDACL
		*currentACE = nil
		return
	case strings.HasPrefix(trimmed, "SACL"):
		*section = sectSACL
		*currentACE = nil
		return
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
		if *section == sectSACL {
			cur.SACL = append(cur.SACL, ace)
			*currentACE = &cur.SACL[len(cur.SACL)-1]
		} else {
			*section = sectDACL
			cur.DACL = append(cur.DACL, ace)
			*currentACE = &cur.DACL[len(cur.DACL)-1]
		}
		return
	}

	// Detail lines: bracketed tokens are flags, bare tokens are rights.
	isFlag := strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")
	token := strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")

	if ace := *currentACE; ace != nil {
		if isFlag {
			ace.AceFlags = append(ace.AceFlags, token)
		} else {
			ace.Rights = append(ace.Rights, trimmed)
		}
		return
	}

	if *section == sectDescriptorFlags {
		cur.DescriptorFlags = append(cur.DescriptorFlags, token)
		return
	}

	// A line indented deeper than the compact access entry it follows is a
	// specific right (only emitted under -v); attach it to the entry.
	if *entryIndent >= 0 && indent > *entryIndent && len(cur.AccessEntries) > 0 {
		last := &cur.AccessEntries[len(cur.AccessEntries)-1]
		last.Rights = append(last.Rights, trimmed)
	}
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
	// descriptorMarker is set during parsing when an SDDL/descriptor marker is
	// seen, so this fallback works even when Raw is omitted for oversized inputs.
	return r.descriptorMarker
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
