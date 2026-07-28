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

package dranzer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/archiveio"
)

// ErrNoReports is returned when a dranzer material holds no recognizable report:
// a single file that is not dranzer output, or an archive with no report entry.
var ErrNoReports = errors.New("no dranzer report found")

// bundleLimits bounds bundle expansion. One dranzer run emits a handful of text
// reports (one per test mode) totalling tens of kilobytes, so this is far tighter
// than the generic archive defaults. It matters because the aggregate is held in
// memory, serialized to JSON, and then parsed into policy-engine values, each a
// multiple of the uncompressed size.
func bundleLimits() archiveio.Limits {
	return archiveio.Limits{MaxEntries: 256, MaxTotalSize: 16 << 20}
}

// Entry is one report's contribution to a Bundle: which archive entry it came
// from and the counters it reported. Its objects and findings are not repeated
// here — they are in the Bundle's aggregate, each stamped with the same Source —
// so the policy input carries them once.
type Entry struct {
	Source  string  `json:"source,omitempty"`
	Tool    Tool    `json:"tool"`
	Summary Summary `json:"summary"`
}

// Bundle is the policy projection of a CERTCC_DRANZER material, which may hold a
// single report or an archive of the per-mode reports produced by one dranzer
// run (its -b, -p, -s and -t modes).
//
// Report is embedded so the aggregate is promoted to the top level of the JSON:
// a policy reading input.summary / input.findings / input.tool behaves the same
// whether the material was one report or a bundle of them. Reports carries the
// per-mode breakdown for policies that need to be precise about which mode
// reported what.
type Bundle struct {
	Report
	Reports []Entry `json:"reports"`
}

// Inspection is what reading a dranzer material tells us about it, without
// retaining its content: how many reports it holds and which tool version
// produced them.
type Inspection struct {
	Reports int
	Version string
}

// Inspect validates a dranzer material on disk and summarizes it, accepting
// either a single report or an archive of them. Archives are streamed rather
// than held in memory.
//
// It shares both of its predicates with ParseBundle — what counts as an archive
// (archiveio.DetectFile, which reads content just as the projection's
// DetectBytes does, rather than trusting the filename) and what counts as a
// report (parseReportEntry) — so a material this accepts is necessarily one the
// projection can aggregate. Were the two to disagree, a material would be
// attested at craft time and then silently skip policy evaluation, which on a
// compliance gate reads as a clean run.
//
// ErrNoReports means the material is not dranzer evidence at all.
func Inspect(p string) (Inspection, error) {
	format, err := archiveio.DetectFile(p)
	if err != nil {
		return Inspection{}, fmt.Errorf("inspecting %q: %w", p, err)
	}

	if format == archiveio.None {
		data, err := os.ReadFile(p)
		if err != nil {
			return Inspection{}, fmt.Errorf("can't open the file: %w", err)
		}
		report, err := Parse(data)
		if err != nil {
			return Inspection{}, err
		}
		if !report.LooksLikeDranzer() {
			return Inspection{}, ErrNoReports
		}
		return Inspection{Reports: 1, Version: report.Tool.Version}, nil
	}

	var out Inspection
	var versionFrom string
	err = archiveio.WalkPath(p, format, bundleLimits(), func(name string, r io.Reader) error {
		report, err := parseReportEntry(r)
		if err != nil || report == nil {
			return err
		}
		out.Reports++
		// ParseBundle sorts entries before folding them, so take the version from
		// the lexicographically first entry that declares one rather than the first
		// the walk happens to reach. Otherwise the recorded annotation and the
		// projected tool.version could disagree for a bundle whose reports were
		// produced by different tool versions.
		if report.Tool.Version != "" && (versionFrom == "" || name < versionFrom) {
			versionFrom, out.Version = name, report.Tool.Version
		}
		return nil
	})
	if err != nil {
		return Inspection{}, fmt.Errorf("reading dranzer archive: %w", err)
	}
	if out.Reports == 0 {
		return Inspection{}, ErrNoReports
	}

	return out, nil
}

// ParseBundle projects one dranzer material to JSON-ready form. A value that is
// not an archive is treated as a bundle of one; an archive is expanded and every
// entry that is a recognizable report contributes to the aggregate.
//
// A single non-archive value is never rejected, so projecting an
// already-recorded material cannot start failing — validating the input is
// Inspect's job, at craft time. An archive, by contrast, must yield at least one
// report: selecting entries is only meaningful if something was selected.
func ParseBundle(data []byte) (*Bundle, error) {
	format := archiveio.DetectBytes(data)
	if format == archiveio.None {
		report, err := Parse(data)
		if err != nil {
			return nil, err
		}
		return newBundle([]Report{*report}), nil
	}

	reports, err := parseArchiveReports(data, format)
	if err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, ErrNoReports
	}

	// Deterministic order so the projection — and therefore the policy input and
	// any decision derived from it — does not depend on archive iteration order.
	sort.Slice(reports, func(i, j int) bool { return reports[i].Source < reports[j].Source })

	return newBundle(reports), nil
}

// parseReportEntry reads one archive entry and returns the report it holds, or
// nil when the entry is not a dranzer report. Bundles ship non-report companion
// files (a CSV summary) beside the reports, so those are skipped rather than
// treated as an error.
//
// This is the single definition of "is this entry a report", shared by Inspect
// and ParseBundle.
func parseReportEntry(r io.Reader) (*Report, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading entry: %w", err)
	}
	report, err := Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parsing entry: %w", err)
	}
	if !report.LooksLikeDranzer() {
		return nil, nil
	}
	return report, nil
}

// parseArchiveReports walks an in-memory archive and returns the entries that are
// dranzer reports, each stamped with its entry name.
func parseArchiveReports(data []byte, format archiveio.Format) ([]Report, error) {
	var reports []Report

	err := archiveio.WalkBytes(data, format, bundleLimits(), func(name string, r io.Reader) error {
		report, err := parseReportEntry(r)
		if err != nil || report == nil {
			return err
		}
		report.Source = name
		reports = append(reports, *report)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading dranzer archive: %w", err)
	}

	return reports, nil
}

// newBundle folds reports into a Bundle: the aggregate at the top level, and the
// per-report breakdown in Reports.
func newBundle(reports []Report) *Bundle {
	nObjects, nFindings, nRaw := 0, 0, 0
	for _, r := range reports {
		nObjects += len(r.Objects)
		nFindings += len(r.Findings)
		nRaw += len(r.Raw) + len(r.Source) + len("=====  =====\n\n")
	}

	b := &Bundle{
		Report: Report{
			Tool:     Tool{Name: ToolName},
			Objects:  make([]Object, 0, nObjects),
			Findings: make([]Finding, 0, nFindings),
			Summary:  Summary{Counters: map[string]int{}},
		},
		Reports: make([]Entry, 0, len(reports)),
	}

	var raw strings.Builder
	raw.Grow(nRaw)

	for _, r := range reports {
		if b.Tool.Version == "" {
			b.Tool.Version = r.Tool.Version
		}

		// Stamp each object and finding with its source so the aggregate stays
		// attributable without repeating them in the per-report breakdown.
		for _, o := range r.Objects {
			o.Source = r.Source
			b.Objects = append(b.Objects, o)
		}
		for _, f := range r.Findings {
			f.Source = r.Source
			b.Findings = append(b.Findings, f)
		}
		b.Summary.add(r.Summary)

		// A single report projects to its own text verbatim; entries in a bundle
		// are attributed and newline-terminated so they cannot run together.
		if r.Source == "" {
			raw.WriteString(r.Raw)
		} else {
			fmt.Fprintf(&raw, "===== %s =====\n%s\n", r.Source, strings.TrimSuffix(r.Raw, "\n"))
		}

		b.Reports = append(b.Reports, Entry{Source: r.Source, Tool: r.Tool, Summary: r.Summary})
	}
	b.Raw = raw.String()

	return b
}

// add accumulates another report's counters into s. Every counter is summed, so
// the aggregate answers "did any report in this bundle see a failure" — which is
// what the compliance gate asks. Note this makes object_count a total across
// reports rather than a count of distinct controls: one run's four modes each
// test the same controls.
func (s *Summary) add(other Summary) {
	if s.Counters == nil {
		s.Counters = map[string]int{}
	}
	for k, v := range other.Counters {
		s.Counters[k] += v
		// Keep the explicit fields in step with the map they mirror.
		if field := s.wellKnownCounter(k); field != nil {
			*field = s.Counters[k]
		}
	}
}
