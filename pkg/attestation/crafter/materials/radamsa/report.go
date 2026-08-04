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

package radamsa

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/archiveio"
)

// ErrInvalidReport marks content that is not valid radamsa -M evidence: a
// malformed log, an archive entry that does not parse, a corrupt gzip stream, or a
// report that exceeds maxReportRecords. It is deliberately distinct from a
// file-access error so a caller can tell "bad evidence" (an invalid material) from
// "couldn't read the file" (a fixable I/O problem).
var ErrInvalidReport = errors.New("invalid radamsa -M report")

// maxReportRecords caps the total number of -M records a report may contribute to
// input.elements. The eval-time projection (ParseReportBytes) must hold every
// record in memory as a map and marshal it to JSON, so an unbounded record count
// would OOM the evaluator. Craft (InspectReport) enforces the SAME cap while only
// streaming and counting, so a report accepted at craft time is guaranteed to be
// evaluable — the two sides share the bound and cannot disagree about it.
const maxReportRecords = 5_000_000

// reportLimits bounds RADAMSA_REPORT archive expansion. The entry cap is far
// higher than archiveio's generic 10,000-entry default because the intended input
// is one -M metadata file per fuzzing run and a campaign routinely has well over
// ten thousand runs, which would otherwise trip the generic cap. The total
// uncompressed-size cap guards against a zip bomb; maxReportRecords separately
// bounds the parsed footprint, which is what the eval projection actually holds.
func reportLimits() archiveio.Limits {
	return archiveio.Limits{MaxEntries: 2_000_000, MaxTotalSize: 1 << 30}
}

// parseEntry parses one -M log. An empty log (zero fuzzing iterations) is valid
// evidence and yields no records; malformed content is ErrInvalidReport. It is the
// single definition of the per-entry parse rule, so InspectReport (craft time) and
// ParseReportBytes (eval time) cannot drift apart on what a report contains — the
// drift that would let a material be attested and then fail or skip its gate at
// eval. This mirrors dranzer.parseReportEntry.
func parseEntry(r io.Reader) ([]map[string]any, error) {
	recs, err := Parse(r)
	switch {
	case errors.Is(err, ErrNoRecords):
		// An empty log is a valid report of zero iterations, not bad evidence; it
		// contributes no records and the gate decides whether zero is acceptable.
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("%w: %w", ErrInvalidReport, err)
	}
	// Enforce the record cap here — the one chokepoint every path flows through —
	// so a single standalone log is bounded exactly as an archive entry is, and no
	// path can hand the eval projection an unbounded record set.
	if len(recs) > maxReportRecords {
		return nil, fmt.Errorf("%w: exceeds %d records", ErrInvalidReport, maxReportRecords)
	}
	return recs, nil
}

// parseSingle parses one standalone -M log and always returns a non-nil slice, so a
// zero-iteration log marshals to `[]` (a real 0 < min violation the gate can read)
// rather than `null` (an absent input.elements the gate would skip on).
func parseSingle(r io.Reader) ([]map[string]any, error) {
	recs, err := parseEntry(r)
	if err != nil {
		return nil, err
	}
	if recs == nil {
		recs = []map[string]any{}
	}
	return recs, nil
}

// isTarStream reports whether r is a tar archive by peeking its first header. An
// empty-but-valid tar (only end-of-archive blocks) reads as io.EOF and still counts
// as a tar; a non-tar stream (e.g. a plain-gzipped -M log) yields a different error.
func isTarStream(r io.Reader) bool {
	_, err := tar.NewReader(r).Next()
	return err == nil || errors.Is(err, io.EOF)
}

// ParseReportBytes parses radamsa -M metadata already held in memory — a single -M
// log, a plain-gzipped log, or an archive (zip/tar/tar.gz) of logs — and returns
// the records merged across every entry. Zero records is a valid, empty result
// (the gate then decides on zero iterations); malformed content is ErrInvalidReport
// and a malformed archive entry fails the whole material rather than being dropped.
//
// This and InspectReport are the single shared definition of what a RADAMSA_REPORT
// material contains — same content detection, per-entry parse, and record cap — so
// the crafter's craft-time validation and the policy engine's eval-time projection
// cannot disagree. Were they to disagree, a material could be attested at craft
// time and then fail or silently skip policy evaluation, which on a compliance gate
// reads as a clean pass.
func ParseReportBytes(data []byte) ([]map[string]any, error) {
	switch format := archiveio.DetectBytes(data); format {
	case archiveio.None:
		return parseSingle(bytes.NewReader(data))
	case archiveio.Zip, archiveio.Tar:
		return mergeArchiveBytes(data, format)
	case archiveio.TarGz:
		// The gzip magic marks both a real tar.gz and a plain-gzipped single -M
		// log; decompress and route on the actual content so a gzipped log is not
		// misread as a (non-)tar and rejected.
		raw, err := gunzip(data)
		if err != nil {
			return nil, err
		}
		if isTarStream(bytes.NewReader(raw)) {
			return mergeArchiveBytes(raw, archiveio.Tar)
		}
		return parseSingle(bytes.NewReader(raw))
	default:
		return nil, fmt.Errorf("%w: unsupported archive format", ErrInvalidReport)
	}
}

// mergeArchiveBytes walks an in-memory archive, parses every entry, and returns the
// records merged in a deterministic (entry-name) order so input.elements does not
// depend on archive iteration order. A malformed entry is fatal.
func mergeArchiveBytes(data []byte, format archiveio.Format) ([]map[string]any, error) {
	type namedRecords struct {
		name string
		recs []map[string]any
	}
	var entries []namedRecords
	total := 0
	err := archiveio.WalkBytes(data, format, reportLimits(), func(name string, r io.Reader) error {
		recs, perr := parseEntry(r)
		if perr != nil {
			return perr
		}
		if len(recs) == 0 {
			return nil
		}
		total += len(recs)
		if total > maxReportRecords {
			return fmt.Errorf("%w: exceeds %d records", ErrInvalidReport, maxReportRecords)
		}
		entries = append(entries, namedRecords{name: name, recs: recs})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading radamsa archive: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	// Non-nil even when empty so a zero-record archive marshals to `[]`, not `null`:
	// the gate reads an empty array (a real 0 < min violation), not an absent field
	// (which it would skip on).
	records := make([]map[string]any, 0, total)
	for _, e := range entries {
		records = append(records, e.recs...)
	}
	return records, nil
}

// InspectReport validates a RADAMSA_REPORT material on disk — a single -M log, a
// plain-gzipped log, or an archive of logs — and returns the total number of
// parsed -M records. Archives are streamed rather than held in memory. It shares
// its content detection, per-entry parse, and record cap with ParseReportBytes (see
// that function) so the two cannot disagree about what a material contains.
//
// Content problems are reported as ErrInvalidReport; file-access errors are
// surfaced as-is so callers can tell an invalid report from an unreadable file.
func InspectReport(path string) (int, error) {
	format, err := archiveio.DetectFile(path)
	if err != nil {
		return 0, fmt.Errorf("inspecting %q: %w", path, err)
	}

	switch format {
	case archiveio.None:
		return countSingle(path)
	case archiveio.Zip, archiveio.Tar:
		return countArchive(path, format)
	case archiveio.TarGz:
		isTar, err := gzipIsTar(path)
		if err != nil {
			return 0, err
		}
		if isTar {
			return countArchive(path, archiveio.TarGz)
		}
		return countGzipSingle(path)
	default:
		return 0, fmt.Errorf("%w: unsupported archive format", ErrInvalidReport)
	}
}

func countSingle(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()
	recs, err := parseEntry(f)
	if err != nil {
		return 0, err
	}
	return len(recs), nil
}

func countGzipSingle(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("can't open the file: %w", err)
	}
	raw, err := gunzip(data)
	if err != nil {
		return 0, err
	}
	recs, err := parseEntry(bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	return len(recs), nil
}

func countArchive(path string, format archiveio.Format) (int, error) {
	total := 0
	err := archiveio.WalkPath(path, format, reportLimits(), func(_ string, r io.Reader) error {
		recs, perr := parseEntry(r)
		if perr != nil {
			return perr
		}
		total += len(recs)
		if total > maxReportRecords {
			return fmt.Errorf("%w: exceeds %d records", ErrInvalidReport, maxReportRecords)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("reading radamsa archive: %w", err)
	}
	return total, nil
}

// gzipIsTar reports whether the gzip stream at path decompresses to a tar (a real
// tar.gz) rather than a plain-gzipped file, by peeking the first tar header. A
// corrupt gzip stream is ErrInvalidReport; a file-access error is surfaced as-is.
func gzipIsTar(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidReport, err)
	}
	defer gz.Close()
	return isTarStream(gz), nil
}

// gunzip decompresses a gzip blob, bounding the output to the archive size limit so
// a gzip bomb cannot exhaust memory. A corrupt or oversized stream is ErrInvalidReport.
func gunzip(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidReport, err)
	}
	defer gz.Close()

	maxSize := reportLimits().MaxTotalSize
	out, err := io.ReadAll(io.LimitReader(gz, maxSize+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidReport, err)
	}
	if int64(len(out)) > maxSize {
		return nil, fmt.Errorf("%w: gzip content exceeds %d bytes", ErrInvalidReport, maxSize)
	}
	return out, nil
}
