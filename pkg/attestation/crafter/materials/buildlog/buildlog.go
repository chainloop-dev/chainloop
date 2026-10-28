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

// Package buildlog extracts the compiler and build diagnostics (warnings and
// errors) from the logs Microsoft build tools write, and normalizes them into
// one Chainloop-defined structure that policies evaluate:
//
//   - MSBuild binary logs (-bl), format version 18 or later
//   - MSBuild text logs (console or file logger, any verbosity that keeps
//     warnings), possibly several invocations appended to one file
//   - Visual C++ build logs (BuildLog.htm, written by vcbuild), UTF-16 HTML
//   - a zip or tar archive holding any mix of the above, in any folders, for
//     builds that produce one log per solution or per project. Archives inside
//     it (such as the binary logs zipped per solution) are read one level deep;
//     entries that are not logs are skipped.
//
// The original log is what a material stores; this projection is recomputed
// from it whenever policies run. It keeps no raw log text: diagnostic-verbosity
// logs reach hundreds of megabytes, and the policy engine holds its whole
// input in memory.
package buildlog

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/archiveio"
)

const (
	FormatMSBuildBinlog = "msbuild-binlog"
	FormatMSBuildText   = "msbuild-text"
	FormatVCBuildHTML   = "vcbuild-html"

	ToolMSBuild = "msbuild"
	ToolVCBuild = "vcbuild"

	SeverityWarning = "warning"
	SeverityError   = "error"

	ResultSucceeded = "succeeded"
	ResultFailed    = "failed"
)

// ErrNoBuildLog is returned when a material holds no recognizable build log: a
// single file in another format, or an archive with no build log entry.
var ErrNoBuildLog = errors.New("no build log found")

// Tool identifies the build tool that wrote a log.
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// Diagnostic is one distinct warning or error. Repeats of the same diagnostic
// (a header warning reported once per translation unit including it, or a
// project built by several solutions) are folded into one entry: Occurrences
// counts them, and Logs lists the indexes in Report.Logs of the logs they were
// seen in, so a folded diagnostic can still be traced to each build.
type Diagnostic struct {
	Severity    string `json:"severity"`
	Code        string `json:"code,omitempty"`
	Message     string `json:"message"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Column      int    `json:"column,omitempty"`
	Project     string `json:"project,omitempty"`
	Occurrences int    `json:"occurrences"`
	Logs        []int  `json:"logs"`
}

// Summary counts distinct diagnostics.
type Summary struct {
	Warnings int            `json:"warnings"`
	Errors   int            `json:"errors"`
	ByCode   map[string]int `json:"by_code"`
}

// Counts are the totals a log states about itself, such as MSBuild's
// "N Warning(s)" summary. They let a policy check that every diagnostic the
// tool reported was parsed.
type Counts struct {
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
}

// Log describes one build recorded in the material: a binary log, one MSBuild
// run of a text log (a text log can hold several, one per solution, which then
// share a Source), or a Visual C++ build log. Source names the archive entry it
// was read from, and is empty for a single-file material. Solution is the
// project the build was run for (usually a .sln), when the log records it.
//
// Result is how the build ended, empty when the log does not say (a log cut
// short). Parsed counts the occurrences read from the log, comparable with the
// totals it Reported about itself: fewer parsed than reported means warnings
// the parser could not read.
type Log struct {
	Source   string  `json:"source,omitempty"`
	Solution string  `json:"solution,omitempty"`
	Format   string  `json:"format"`
	Tool     Tool    `json:"tool"`
	Result   string  `json:"result,omitempty"`
	Summary  Summary `json:"summary"`
	Parsed   Counts  `json:"parsed"`
	Reported *Counts `json:"reported,omitempty"`
}

// Report is the policy projection of a BUILD_LOG material. Skipped lists the
// archive entries that were not read as logs, so a policy can tell a log that
// could not be read from a file that is not a log.
type Report struct {
	Tool        Tool         `json:"tool"`
	Logs        []Log        `json:"logs"`
	Skipped     []string     `json:"skipped,omitempty"`
	Summary     Summary      `json:"summary"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// parsedLog is one log as read, before its diagnostics are folded.
type parsedLog struct {
	source      string
	solution    string
	result      string
	format      string
	tool        Tool
	diagnostics []Diagnostic
	reported    *Counts
}

// archiveLimits bounds archive expansion. Logs are streamed and only their
// diagnostics are kept, so the bound is on the work done, not on memory.
func archiveLimits() archiveio.Limits {
	return archiveio.Limits{MaxEntries: 1024, MaxTotalSize: 4 << 30}
}

// Parse projects a BUILD_LOG material held in memory.
func Parse(data []byte) (*Report, error) {
	if isBinlog(data) {
		return parseSingle(bytes.NewReader(data))
	}
	if format := archiveio.DetectBytes(data); format != archiveio.None {
		return parseArchive(func(yield func(string, io.Reader) error) error {
			return archiveio.WalkBytes(data, format, archiveLimits(), yield)
		})
	}
	return parseSingle(bytes.NewReader(data))
}

// ParseFile projects a BUILD_LOG material on disk, streaming it.
func ParseFile(p string) (*Report, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()

	br := bufio.NewReader(f)
	peek, _ := br.Peek(512)
	if isBinlog(peek) {
		return parseSingle(br)
	}
	format, err := archiveio.DetectFile(p)
	if err != nil {
		return nil, fmt.Errorf("inspecting %q: %w", p, err)
	}
	if format != archiveio.None {
		return parseArchive(func(yield func(string, io.Reader) error) error {
			return archiveio.WalkPath(p, format, archiveLimits(), yield)
		})
	}
	return parseSingle(br)
}

func parseSingle(r io.Reader) (*Report, error) {
	logs, err := parseLog(r)
	if err != nil {
		return nil, err
	}
	if len(logs) == 0 {
		return nil, ErrNoBuildLog
	}
	return newReport(logs), nil
}

// walker yields the entries of an archive.
type walker func(yield func(name string, r io.Reader) error) error

// maxNestedArchive bounds an archive inside the material's archive, which has
// to be read into memory to be opened.
const maxNestedArchive = 512 << 20

// nestedArchiveExts are the entry names opened as archives. Packages such as
// .nupkg or .vsix are zips too, but not bundles of logs, so they are skipped
// like any other entry that is not a log.
var nestedArchiveExts = []string{".zip", ".tar", ".tar.gz", ".tgz"}

func isArchiveName(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range nestedArchiveExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// parseArchive parses every archive entry that is a build log, including those
// of an archive one level down: a bundle of everything a release produced may
// hold the binary logs already zipped per solution. Other entries are skipped,
// but at least one log must be found.
func parseArchive(walk walker) (*Report, error) {
	c := collected{budget: budget{limits: archiveLimits()}}
	if err := c.walk(walk, "", 0); err != nil {
		return nil, fmt.Errorf("reading build log archive: %w", err)
	}
	if len(c.logs) == 0 {
		return nil, ErrNoBuildLog
	}
	// Deterministic order, so the projection does not depend on archive order.
	// Stable, so the runs of one text log keep their order.
	sort.SliceStable(c.logs, func(i, j int) bool { return c.logs[i].source < c.logs[j].source })
	sort.Strings(c.skipped)
	report := newReport(c.logs)
	report.Skipped = c.skipped
	return report, nil
}

// collected gathers the logs of an archive and the entries that are not logs.
type collected struct {
	logs    []*parsedLog
	skipped []string
	budget  budget
}

// budget counts the entries and bytes read from the material's archive and
// from the archives inside it. Each archive walk applies its limits to itself
// only, so without a shared count every nested archive could expand to the
// full limits again.
type budget struct {
	limits  archiveio.Limits
	entries int
	read    int64
}

// entry counts an entry and returns a reader that counts its bytes.
func (b *budget) entry(r io.Reader) (io.Reader, error) {
	b.entries++
	if b.entries > b.limits.MaxEntries {
		return nil, archiveio.ErrTooManyEntries
	}
	return &budgetReader{r: r, b: b}, nil
}

type budgetReader struct {
	r io.Reader
	b *budget
}

func (br *budgetReader) Read(p []byte) (int, error) {
	n, err := br.r.Read(p)
	br.b.read += int64(n)
	if br.b.read > br.b.limits.MaxTotalSize {
		return n, archiveio.ErrArchiveTooLarge
	}
	return n, err
}

// projectImportsSuffix names the archive of project files that
// -bl:ProjectImports=ZipFile writes next to a binary log. It never holds logs.
const projectImportsSuffix = ".projectimports.zip"

// walk adds the build logs of an archive, naming each by its path from the
// material's archive. depth counts the archives the walk is in beyond the
// material's own.
func (c *collected) walk(walk walker, prefix string, depth int) error {
	return walk(func(name string, r io.Reader) error {
		source := prefix + name
		r, err := c.budget.entry(r)
		if err != nil {
			return err
		}
		switch {
		case strings.HasSuffix(strings.ToLower(name), projectImportsSuffix):
			c.skipped = append(c.skipped, source)
			return nil
		case isArchiveName(name):
			return c.nested(r, source, depth)
		}
		return c.add(r, source)
	})
}

// add parses r and adds its logs, or records it as skipped when it holds none.
func (c *collected) add(r io.Reader, source string) error {
	parsed, err := parseLog(r)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", source, err)
	}
	if len(parsed) == 0 {
		c.skipped = append(c.skipped, source)
		return nil
	}
	for _, l := range parsed {
		l.source = source
	}
	c.logs = append(c.logs, parsed...)
	return nil
}

// nested opens an archive found inside the material's archive. Skipping it
// would drop its logs from the evaluation without a trace, so anything that
// cannot be read fails instead.
func (c *collected) nested(r io.Reader, source string, depth int) error {
	if depth > 0 {
		return fmt.Errorf("archive %q is inside another archive within the material: move its logs up one level", source)
	}
	data, err := io.ReadAll(io.LimitReader(r, maxNestedArchive+1))
	if err != nil {
		return fmt.Errorf("reading %q: %w", source, err)
	}
	if len(data) > maxNestedArchive {
		return fmt.Errorf("archive %q is larger than %d MiB: add its logs to the material's archive directly", source, maxNestedArchive>>20)
	}

	format := archiveio.DetectBytes(data)
	if format == archiveio.None || isBinlog(data) {
		// Named like an archive but is not one; it may still be a log.
		return c.add(bytes.NewReader(data), source)
	}
	return c.walk(func(yield func(string, io.Reader) error) error {
		return archiveio.WalkBytes(data, format, archiveLimits(), yield)
	}, source+"/", depth+1)
}

// sniffSize is how much of a file is read to recognize it. A CI console capture
// can hold the output of earlier steps before MSBuild starts.
const sniffSize = 64 << 10

// parseLog parses the builds recorded in one log file, or returns none when r
// is not a build log.
func parseLog(r io.Reader) ([]*parsedLog, error) {
	br := bufio.NewReaderSize(r, sniffSize)
	peek, err := br.Peek(sniffSize)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return nil, err
	}
	if isBinlog(peek) {
		log, err := parseBinlog(br)
		if err != nil {
			return nil, err
		}
		return []*parsedLog{log}, nil
	}
	if format := sniffText(peek); format != "" {
		return parseText(br, format)
	}
	return nil, nil
}

// diagnosticKey identifies a diagnostic for folding repeats. Paths are compared
// without regard to case, as the Windows tools that write these logs spell the
// same file differently from one report to the next.
type diagnosticKey struct {
	severity, code, message, file, project string
	line, column                           int
}

func keyOf(d Diagnostic) diagnosticKey {
	return diagnosticKey{
		severity: d.Severity,
		code:     d.Code,
		message:  d.Message,
		file:     strings.ToLower(d.File),
		project:  strings.ToLower(d.Project),
		line:     d.Line,
		column:   d.Column,
	}
}

// fold merges repeated diagnostics, keeping the first spelling of each, and
// sorts the result.
func fold(diagnostics []Diagnostic) []Diagnostic {
	index := make(map[diagnosticKey]int, len(diagnostics))
	out := make([]Diagnostic, 0, len(diagnostics))
	for _, d := range diagnostics {
		k := keyOf(d)
		if i, ok := index[k]; ok {
			out[i].Occurrences += max(d.Occurrences, 1)
			out[i].Logs = mergeIndexes(out[i].Logs, d.Logs)
			continue
		}
		d.Occurrences = max(d.Occurrences, 1)
		index[k] = len(out)
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Project != b.Project {
			return a.Project < b.Project
		}
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		return a.Message < b.Message
	})
	return out
}

// occurrences counts the diagnostics read from a log, before folding.
func occurrences(diagnostics []Diagnostic) Counts {
	var c Counts
	for _, d := range diagnostics {
		if d.Severity == SeverityError {
			c.Errors += max(d.Occurrences, 1)
		} else {
			c.Warnings += max(d.Occurrences, 1)
		}
	}
	return c
}

// mergeIndexes returns the sorted union of two index lists.
func mergeIndexes(a, b []int) []int {
	out := append(slices.Clone(a), b...)
	slices.Sort(out)
	return slices.Compact(out)
}

func summarize(diagnostics []Diagnostic) Summary {
	s := Summary{ByCode: map[string]int{}}
	for _, d := range diagnostics {
		if d.Severity == SeverityError {
			s.Errors++
		} else {
			s.Warnings++
		}
		if d.Code != "" {
			s.ByCode[d.Code]++
		}
	}
	return s
}

func newReport(logs []*parsedLog) *Report {
	report := &Report{Logs: make([]Log, 0, len(logs))}
	var all []Diagnostic
	for i, l := range logs {
		folded := fold(l.diagnostics)
		for j := range folded {
			folded[j].Logs = []int{i}
		}
		if report.Tool.Name == "" {
			report.Tool = l.tool
		}
		report.Logs = append(report.Logs, Log{
			Source:   l.source,
			Solution: l.solution,
			Result:   l.result,
			Parsed:   occurrences(l.diagnostics),
			Format:   l.format,
			Tool:     l.tool,
			Summary:  summarize(folded),
			Reported: l.reported,
		})
		all = append(all, folded...)
	}
	report.Diagnostics = fold(all)
	report.Summary = summarize(report.Diagnostics)
	return report
}
