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

// Package dranzer parses the plain-text report produced by the CERT/CC dranzer
// tool (https://github.com/CERTCC/dranzer), which fuzz-tests ActiveX/COM
// controls. Dranzer has no machine-readable output mode and its format is
// undocumented, so the parser is intentionally tolerant: it extracts the
// structure it recognizes (the run summary, per-object metadata and error
// findings) and always preserves the full original text in Raw so a policy can
// fall back to string matching.
//
// Real dranzer reports are emitted in the system's ANSI code page rather than
// UTF-8, so the parser sanitizes invalid byte sequences instead of rejecting
// them.
package dranzer

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ToolName is the canonical tool name recorded for dranzer materials.
const ToolName = "dranzer"

var (
	// objectRe matches a per-object banner such as
	// "Testing COM Object - {GUID} <description>". The CLSID may or may not be
	// wrapped in braces; the optional trailing text is the control description.
	objectRe = regexp.MustCompile(`^Testing COM Object - (\{[^}]+\}|[0-9A-Fa-f-]+)\s*(.*)$`)
	// failureHeaderRe matches the "{GUID}-<class name>" line that introduces a
	// failed-object block in the report header, ahead of its "ERROR - ..." line.
	// dranzer emits "%ws-%s", so the description may be empty ("{GUID}-").
	failureHeaderRe = regexp.MustCompile(`^(\{[^}]+\})-(.*)$`)
	// errorRe matches an error line "ERROR - <message> (0x<code>)".
	errorRe = regexp.MustCompile(`^ERROR - (.*?)\s*\((0x[0-9A-Fa-f]+)\)\s*$`)
	// methodRe matches the "Invoking|Invoked <type> - Interface::Method" lines,
	// where <type> is a space-padded token (Method / Property Get / Property Put
	// / Property Put Reference). The captured signature gives context to the
	// finding emitted by the access-violation/exception handler that follows.
	methodRe = regexp.MustCompile(`^Invok(?:ing|ed) (?:Method|Property Get|Property Put Reference|Property Put)\s+-\s+(.+)$`)
	// avRe matches the access-violation detail line
	// "Access violation at 0x<addr> :Bad <read|write> on 0x<addr>".
	avRe = regexp.MustCompile(`^(.+?) at (0x[0-9A-Fa-f]+) :Bad (read|write) on (0x[0-9A-Fa-f]+)$`)
	// win32Re matches the Win32-exception detail line
	// "<description> (code <hex>) at 0x<addr>".
	win32Re = regexp.MustCompile(`^(.+?) \(code ([0-9A-Fa-f]+)\) at (0x[0-9A-Fa-f]+)$`)
	// metadataRe matches a "Key : value" / "Key    : value" metadata line; dranzer
	// pads the key with a variable number of spaces before the colon.
	metadataRe = regexp.MustCompile(`^([A-Za-z][A-Za-z .]+?)\s*:\s*(.*)$`)
	// counterRe matches a summary counter "Number of <label>   <n>".
	counterRe = regexp.MustCompile(`^Number of (.+?)\s{2,}([0-9]+)$`)
	// versionRe extracts the test engine revision, e.g. "Test Engine Version: $Rev: 96 $".
	versionRe = regexp.MustCompile(`(?i)Test Engine Version:\s*\$Rev:\s*([0-9]+)`)
)

// Tool holds the tool identity recorded for a dranzer report.
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// Finding is a single error reported against a COM object during the run. The
// header failure blocks populate CLSID/ClassName/ErrorCode/ErrorMessage; the
// inline access-violation and exception blocks additionally populate Method,
// Address and AccessType.
type Finding struct {
	CLSID        string `json:"clsid,omitempty"`
	ClassName    string `json:"class_name,omitempty"`
	Method       string `json:"method,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	Address      string `json:"address,omitempty"`
	AccessType   string `json:"access_type,omitempty"`
}

// Object is a single COM/ActiveX control described in the report, with its
// version/identity metadata. Only the per-object test modes (e.g. -t) emit
// these blocks; summary-only modes (-b/-p/-s) leave Objects empty.
type Object struct {
	CLSID       string            `json:"clsid,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Summary holds the run-level counters dranzer prints in every report. The
// well-known counters are exposed as explicit fields for convenient policy
// access; every "Number of ..." line is also recorded verbatim (normalized to a
// snake_case key) in Counters so mode-specific counters are not lost.
type Summary struct {
	ObjectCount int            `json:"object_count"`
	KillBit     int            `json:"kill_bit_count"`
	Passed      int            `json:"passed_count"`
	Failed      int            `json:"failed_count"`
	Hung        int            `json:"hung_count"`
	Counters    map[string]int `json:"counters,omitempty"`
}

// Report is the structured projection of a dranzer run.
type Report struct {
	Tool     Tool      `json:"tool"`
	Objects  []Object  `json:"objects"`
	Findings []Finding `json:"findings"`
	Summary  Summary   `json:"summary"`
	Raw      string    `json:"raw"`
}

// Parse converts a dranzer text report into a Report. Real reports are emitted
// in the system ANSI code page, so invalid UTF-8 byte sequences are sanitized
// rather than rejected; parsing therefore never fails on well-formed reports.
// Unrecognized content is preserved in the top-level Raw field.
func Parse(data []byte) (*Report, error) {
	// dranzer writes in the system ANSI code page (e.g. ISO-8859-1), so the
	// input is frequently not valid UTF-8. Drop invalid byte sequences so the
	// text projects cleanly to JSON while keeping the recognizable content.
	raw := strings.ToValidUTF8(string(data), "")

	report := &Report{
		Tool:     Tool{Name: ToolName},
		Objects:  []Object{},
		Findings: []Finding{},
		Summary:  Summary{Counters: map[string]int{}},
		Raw:      raw,
	}

	if m := versionRe.FindStringSubmatch(raw); m != nil {
		report.Tool.Version = m[1]
	}

	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	var current *Object
	var pending *Finding
	var lastMethod string

	for _, line := range strings.Split(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isSeparatorLine(trimmed) {
			continue
		}

		// Summary counter line, e.g. "Number of COM Objects Failed Test   1".
		if m := counterRe.FindStringSubmatch(trimmed); m != nil {
			report.applyCounter(m[1], atoi(m[2]))
			continue
		}

		// Object banner starts a new object and ends any pending failure header.
		if m := objectRe.FindStringSubmatch(trimmed); m != nil {
			report.Objects = append(report.Objects, Object{
				CLSID:       m[1],
				Description: strings.TrimSpace(m[2]),
				Metadata:    map[string]string{},
			})
			current = &report.Objects[len(report.Objects)-1]
			pending = nil
			lastMethod = ""
			continue
		}

		// Method/property invocation line: remember the signature so the
		// access-violation or exception detail line that follows can reference it.
		if m := methodRe.FindStringSubmatch(trimmed); m != nil {
			lastMethod = strings.TrimSpace(m[1])
			continue
		}

		// Inline access-violation detail line.
		if m := avRe.FindStringSubmatch(trimmed); m != nil {
			report.Findings = append(report.Findings, inlineFinding(current, Finding{
				Method:       lastMethod,
				ErrorMessage: strings.TrimSpace(m[1]),
				Address:      m[2],
				AccessType:   m[3],
			}))
			continue
		}

		// Inline Win32-exception detail line.
		if m := win32Re.FindStringSubmatch(trimmed); m != nil {
			report.Findings = append(report.Findings, inlineFinding(current, Finding{
				Method:       lastMethod,
				ErrorMessage: strings.TrimSpace(m[1]),
				ErrorCode:    "0x" + m[2],
				Address:      m[3],
			}))
			continue
		}

		// "{GUID}-<class name>" header introducing a failed-object error block.
		if m := failureHeaderRe.FindStringSubmatch(trimmed); m != nil {
			report.Findings = append(report.Findings, Finding{CLSID: m[1], ClassName: strings.TrimSpace(m[2])})
			pending = &report.Findings[len(report.Findings)-1]
			continue
		}

		// Error line: complete a pending failure header, or attach to the
		// current object, or record a standalone finding.
		if m := errorRe.FindStringSubmatch(trimmed); m != nil {
			msg, code := m[1], m[2]
			switch {
			case pending != nil:
				pending.ErrorMessage, pending.ErrorCode = msg, code
				pending = nil
			case current != nil:
				report.Findings = append(report.Findings, Finding{
					CLSID: current.CLSID, ClassName: current.Description,
					ErrorMessage: msg, ErrorCode: code,
				})
			default:
				report.Findings = append(report.Findings, Finding{ErrorMessage: msg, ErrorCode: code})
			}
			continue
		}

		// The version banner marks the start of the trailing summary section, so
		// no further lines belong to an object.
		if strings.HasPrefix(trimmed, "Test Engine Version") {
			current = nil
			continue
		}

		// Per-object metadata "Key : value" lines.
		if current != nil {
			if m := metadataRe.FindStringSubmatch(trimmed); m != nil {
				current.Metadata[normalizeKey(m[1])] = strings.TrimSpace(m[2])
			}
		}
	}

	return report, nil
}

// inlineFinding stamps an inline (access-violation/exception) finding with the
// CLSID and description of the object currently being tested, when known.
func inlineFinding(current *Object, f Finding) Finding {
	if current != nil {
		f.CLSID = current.CLSID
		f.ClassName = current.Description
	}
	return f
}

// applyCounter records a summary counter both in the explicit field that maps to
// its well-known label and, always, in the Counters map under a normalized key.
func (r *Report) applyCounter(label string, value int) {
	label = strings.TrimSpace(label)
	r.Summary.Counters[normalizeKey(label)] = value

	switch strings.ToLower(label) {
	case "com objects":
		r.Summary.ObjectCount = value
	case "com objects with kill bit":
		r.Summary.KillBit = value
	case "com objects passed test":
		r.Summary.Passed = value
	case "com objects failed test":
		r.Summary.Failed = value
	case "com objects hung during test":
		r.Summary.Hung = value
	}
}

// normalizeKey turns a human label such as "COM Object Filename" into a stable
// snake_case key ("com_object_filename") suitable for policy lookups.
func normalizeKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, ".", "")
	return strings.Join(strings.Fields(s), "_")
}

// atoi parses a non-negative integer, returning 0 on failure. dranzer's summary
// counters are always well-formed, so this keeps the caller simple.
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

// isSeparatorLine reports whether a trimmed line is a banner/separator rule
// (e.g. "*****" or "*** Access Violation ***") rather than content.
func isSeparatorLine(trimmed string) bool {
	stripped := strings.Trim(trimmed, "* ")
	return stripped == "" || strings.EqualFold(stripped, "Access Violation")
}

// LooksLikeDranzer reports whether the parsed report resembles genuine dranzer
// output. It is deliberately lenient: the test-engine version banner, a parsed
// object or finding, or the recognizable run-summary line is enough.
func (r *Report) LooksLikeDranzer() bool {
	if r.Tool.Version != "" || len(r.Objects) > 0 || len(r.Findings) > 0 {
		return true
	}
	return strings.Contains(r.Raw, "Testing COM Object -") || strings.Contains(r.Raw, "Number of COM Objects")
}

// JSON returns the report serialized as JSON for the policy engine.
func (r *Report) JSON() ([]byte, error) {
	return json.Marshal(r)
}
