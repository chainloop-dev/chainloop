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
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/archiveio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bundleDir = "../testdata/dranzer-bundle/"

// bundleFiles are the entries of the reference bundle: one report per
// dranzer test mode plus the CSV companion that is not a report.
var bundleFiles = []string{
	"example-app_1.0.0_b_Result.txt",
	"example-app_1.0.0_p_Result.txt",
	"example-app_1.0.0_s_Result.txt",
	"example-app_1.0.0_t_Result.txt",
	"checkResult_Dranzer.csv",
}

func readBundleFiles(t *testing.T, names []string) map[string][]byte {
	t.Helper()
	out := make(map[string][]byte, len(names))
	for _, n := range names {
		data, err := os.ReadFile(bundleDir + n)
		require.NoError(t, err)
		out["Dranzer/"+n] = data
	}
	return out
}

func zipOf(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// A real bundle carries a directory entry; include it so it is exercised.
	_, err := zw.Create("Dranzer/")
	require.NoError(t, err)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func tarGzOf(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name, Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(content)),
		}))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// assertBundleAggregate pins the aggregate of the four mode reports.
// The same 4 COM controls are tested in each mode, so object_count sums to 16
// rather than 4: the aggregate reports totals across the bundle's reports, and
// failed_count summing to 1 is what makes the gate fire.
func assertBundleAggregate(t *testing.T, b *Bundle) {
	t.Helper()
	assert.Equal(t, ToolName, b.Tool.Name)
	assert.Equal(t, "96", b.Tool.Version)
	assert.Equal(t, 16, b.Summary.ObjectCount)
	assert.Equal(t, 15, b.Summary.Passed)
	assert.Equal(t, 1, b.Summary.Failed)
	assert.Equal(t, 0, b.Summary.Hung)
	assert.Equal(t, 0, b.Summary.KillBit)
	assert.Equal(t, 16, b.Summary.Counters["com_objects_without_kill_bit"])
	// Only the -t mode emits per-object blocks and the failure finding.
	assert.Len(t, b.Objects, 1)
	assert.Len(t, b.Findings, 1)
}

func TestParseBundleZip(t *testing.T) {
	data := zipOf(t, readBundleFiles(t, bundleFiles))

	b, err := ParseBundle(data)
	require.NoError(t, err)

	assertBundleAggregate(t, b)

	// The CSV companion is excluded, leaving exactly the four mode reports,
	// ordered deterministically by entry name.
	require.Len(t, b.Reports, 4)
	assert.Equal(t, []string{
		"Dranzer/example-app_1.0.0_b_Result.txt",
		"Dranzer/example-app_1.0.0_p_Result.txt",
		"Dranzer/example-app_1.0.0_s_Result.txt",
		"Dranzer/example-app_1.0.0_t_Result.txt",
	}, []string{b.Reports[0].Source, b.Reports[1].Source, b.Reports[2].Source, b.Reports[3].Source})
}

func TestParseBundleTarGz(t *testing.T) {
	data := tarGzOf(t, readBundleFiles(t, bundleFiles))

	b, err := ParseBundle(data)
	require.NoError(t, err)

	assertBundleAggregate(t, b)
	assert.Len(t, b.Reports, 4)
}

func TestParseBundleStampsFindingSource(t *testing.T) {
	data := zipOf(t, readBundleFiles(t, bundleFiles))

	b, err := ParseBundle(data)
	require.NoError(t, err)

	// The single finding must be traceable to the report it came from.
	require.Len(t, b.Findings, 1)
	assert.Equal(t, "Dranzer/example-app_1.0.0_t_Result.txt", b.Findings[0].Source)
	assert.Equal(t, "0xe0434352", b.Findings[0].ErrorCode)
}

func TestParseBundleRawHoldsEveryReportOnce(t *testing.T) {
	data := zipOf(t, readBundleFiles(t, bundleFiles))

	b, err := ParseBundle(data)
	require.NoError(t, err)

	// The aggregate Raw keeps every report's text for string-matching policies,
	// each attributed to its source.
	for _, r := range b.Reports {
		assert.Contains(t, b.Raw, r.Source)
	}
	assert.Contains(t, b.Raw, "Example.WidgetControl")
	// The CSV is not a report, so its content must not leak into the aggregate.
	assert.NotContains(t, b.Raw, "Detail Information")

	// The aggregate is the only place the text lives: reports[] carries counters,
	// not content, so the policy input never holds a report twice.
	out, err := json.Marshal(b)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(out), `"raw"`),
		"report text must be serialized exactly once")
}

func TestParseBundleBreakdownKeepsPerModeCounters(t *testing.T) {
	data := zipOf(t, readBundleFiles(t, bundleFiles))

	b, err := ParseBundle(data)
	require.NoError(t, err)

	// The per-mode counters are what the breakdown exists for: three clean modes
	// and the -t mode that failed one control.
	require.Len(t, b.Reports, 4)
	failed := make([]int, 0, len(b.Reports))
	for _, e := range b.Reports {
		failed = append(failed, e.Summary.Failed)
		assert.Equal(t, ToolName, e.Tool.Name)
	}
	assert.Equal(t, []int{0, 0, 0, 1}, failed)
}

func TestParseBundleSingleReport(t *testing.T) {
	data, err := os.ReadFile(bundleDir + "example-app_1.0.0_t_Result.txt")
	require.NoError(t, err)

	b, err := ParseBundle(data)
	require.NoError(t, err)

	// A non-archive value behaves as a bundle of one, so policies can iterate
	// reports uniformly. The aggregate equals the single report.
	assert.Equal(t, "96", b.Tool.Version)
	assert.Equal(t, 4, b.Summary.ObjectCount)
	assert.Equal(t, 1, b.Summary.Failed)
	assert.Len(t, b.Findings, 1)
	require.Len(t, b.Reports, 1)
	assert.Empty(t, b.Reports[0].Source, "a single report has no archive entry name")
	assert.NotEmpty(t, b.Raw)
}

func TestParseBundleSingleNonReportIsNotRejected(t *testing.T) {
	// Backward compatibility: the projection of an already-recorded single-file
	// material must not start failing. Validation is the crafter's job.
	b, err := ParseBundle([]byte("not a dranzer report at all"))

	require.NoError(t, err)
	assert.False(t, b.LooksLikeDranzer())
	assert.Len(t, b.Reports, 1)
}

// TestInspectAgreesWithParseBundle pins the invariant the two entry points exist
// to uphold: whatever craft time accepts, evaluation time must be able to
// aggregate. A material accepted by Inspect whose ParseBundle projection is
// unrecognizable would be attested and then silently *skip* policy evaluation,
// which on a compliance gate reads as a clean run.
//
// The prepended-stub case is the one that matters: a zip's central directory sits
// at the end, so a reader still opens it, but its leading bytes are no longer the
// zip magic. Detecting the container by filename at craft time and by content at
// evaluation time would disagree on exactly that input.
func TestInspectAgreesWithParseBundle(t *testing.T) {
	reports := readBundleFiles(t, bundleFiles)

	write := func(t *testing.T, name string, data []byte) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(p, data, 0o600))
		return p
	}

	singleReport, err := os.ReadFile(bundleDir + "example-app_1.0.0_t_Result.txt")
	require.NoError(t, err)

	testCases := []struct {
		name string
		file string
		data []byte
	}{
		{"bundle zip", "Dranzer.zip", zipOf(t, reports)},
		{"bundle tar.gz", "Dranzer.tar.gz", tarGzOf(t, reports)},
		{"single report", "report.txt", singleReport},
		{"zip with a prepended stub", "Dranzer.zip", append([]byte("MZ-self-extracting-stub-"), zipOf(t, reports)...)},
		{"zip of only the CSV companion", "Dranzer.zip", zipOf(t, readBundleFiles(t, []string{"checkResult_Dranzer.csv"}))},
		{"not an archive and not a report", "notes.zip", []byte("just some text")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inspection, inspectErr := Inspect(write(t, tc.file, tc.data))
			if inspectErr != nil {
				// Rejected at craft time; nothing is recorded, so there is no
				// projection to disagree with.
				return
			}

			bundle, err := ParseBundle(tc.data)
			require.NoError(t, err, "Inspect accepted this material, so ParseBundle must project it")
			assert.True(t, bundle.LooksLikeDranzer(),
				"Inspect accepted this material, so its projection must be recognizable or the policy silently skips")
			assert.Equal(t, inspection.Reports, len(bundle.Reports),
				"craft time and evaluation time must find the same number of reports")
		})
	}
}

// TestBundleLimitsAreEnforced pins that both entry points bound expansion with
// the dranzer-specific limits rather than the far looser generic defaults. A
// dranzer bundle is a handful of small text reports, and the aggregate is held in
// memory, serialized to JSON, then parsed into policy-engine values — so the
// generic 10000-entry allowance would let a hostile archive cost orders of
// magnitude more than any real input.
func TestBundleLimitsAreEnforced(t *testing.T) {
	limits := bundleLimits()

	// One entry beyond the cap. Under the generic defaults this would walk fine.
	entries := make(map[string][]byte, limits.MaxEntries+1)
	report, err := os.ReadFile(bundleDir + "example-app_1.0.0_b_Result.txt")
	require.NoError(t, err)
	for i := 0; i <= limits.MaxEntries; i++ {
		entries[fmt.Sprintf("Dranzer/report%d.txt", i)] = report
	}
	data := zipOf(t, entries)

	t.Run("ParseBundle", func(t *testing.T) {
		_, err := ParseBundle(data)
		assert.ErrorIs(t, err, archiveio.ErrTooManyEntries)
	})

	t.Run("Inspect", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "Dranzer.zip")
		require.NoError(t, os.WriteFile(p, data, 0o600))

		_, err := Inspect(p)
		assert.ErrorIs(t, err, archiveio.ErrTooManyEntries)
	})
}

func TestParseBundleArchiveWithNoReports(t *testing.T) {
	data := zipOf(t, readBundleFiles(t, []string{"checkResult_Dranzer.csv"}))

	_, err := ParseBundle(data)

	assert.ErrorIs(t, err, ErrNoReports)
}

func TestParseBundleJSONShapeIsBackwardCompatible(t *testing.T) {
	data, err := os.ReadFile(bundleDir + "example-app_1.0.0_t_Result.txt")
	require.NoError(t, err)

	b, err := ParseBundle(data)
	require.NoError(t, err)

	out, err := json.Marshal(b)
	require.NoError(t, err)

	// The aggregate is promoted to the top level so existing policies that read
	// input.summary / input.findings / input.tool keep working unchanged, with
	// reports[] added alongside.
	var decoded struct {
		Tool     Tool      `json:"tool"`
		Summary  Summary   `json:"summary"`
		Findings []Finding `json:"findings"`
		Raw      string    `json:"raw"`
		Reports  []Entry   `json:"reports"`
	}
	require.NoError(t, json.Unmarshal(out, &decoded))

	assert.Equal(t, "dranzer", decoded.Tool.Name)
	assert.Equal(t, 1, decoded.Summary.Failed)
	assert.Len(t, decoded.Findings, 1)
	assert.NotEmpty(t, decoded.Raw)
	assert.Len(t, decoded.Reports, 1)
}
