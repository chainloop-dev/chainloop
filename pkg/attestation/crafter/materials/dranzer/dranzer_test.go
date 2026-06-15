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
	"os"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		name             string
		file             string
		wantLooksLike    bool
		wantVersion      string
		wantObjectCount  int
		wantPassed       int
		wantFailed       int
		wantFindingCount int
	}{
		{
			name:             "per-object report with a failed object",
			file:             "testdata/dranzer-report.txt",
			wantLooksLike:    true,
			wantVersion:      "96",
			wantObjectCount:  2,
			wantPassed:       1,
			wantFailed:       1,
			wantFindingCount: 1,
		},
		{
			name:            "summary-only report with no findings",
			file:            "testdata/dranzer-summary.txt",
			wantLooksLike:   true,
			wantVersion:     "96",
			wantObjectCount: 2,
			wantPassed:      2,
			wantFailed:      0,
		},
		{
			name:          "non-dranzer text parses but is not recognized",
			file:          "testdata/garbage.txt",
			wantLooksLike: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.file)
			require.NoError(t, err)

			report, err := Parse(data)
			require.NoError(t, err)

			assert.Equal(t, tc.wantLooksLike, report.LooksLikeDranzer())
			assert.Equal(t, ToolName, report.Tool.Name)
			assert.Equal(t, tc.wantVersion, report.Tool.Version)
			// ObjectCount comes from the summary counter, present in every mode.
			assert.Equal(t, tc.wantObjectCount, report.Summary.ObjectCount)
			assert.Equal(t, tc.wantPassed, report.Summary.Passed)
			assert.Equal(t, tc.wantFailed, report.Summary.Failed)
			assert.Len(t, report.Findings, tc.wantFindingCount)

			// Raw is always preserved and valid UTF-8 for the policy projection.
			assert.True(t, utf8.ValidString(report.Raw))
		})
	}
}

func TestParseExtractsFindingAndMetadata(t *testing.T) {
	data, err := os.ReadFile("testdata/dranzer-report.txt")
	require.NoError(t, err)

	report, err := Parse(data)
	require.NoError(t, err)

	// The header error block is captured as a finding with its CLSID and code.
	require.Len(t, report.Findings, 1)
	finding := report.Findings[0]
	assert.Equal(t, "{11111111-2222-3333-4444-555555555555}", finding.CLSID)
	assert.Equal(t, "Example.WidgetControl", finding.ClassName)
	assert.Equal(t, "0xe0434352", finding.ErrorCode)
	assert.Equal(t, "[Unknown Error]", finding.ErrorMessage)

	// The "Testing COM Object" block is parsed into an object with metadata.
	require.Len(t, report.Objects, 1)
	obj := report.Objects[0]
	assert.Equal(t, "{11111111-2222-3333-4444-555555555555}", obj.CLSID)
	assert.Equal(t, "Example.WidgetControl", obj.Description)
	assert.Equal(t, "example.ocx", obj.Metadata["com_object_filename"])
	assert.Equal(t, "Example Corp", obj.Metadata["company_name"])

	// Mode-specific counters are preserved verbatim in the Counters map.
	assert.Equal(t, 0, report.Summary.Counters["com_objects_not_script_safe"])
}

func TestParseCapturesInlineAccessViolation(t *testing.T) {
	data, err := os.ReadFile("testdata/dranzer-crash.txt")
	require.NoError(t, err)

	report, err := Parse(data)
	require.NoError(t, err)

	require.Len(t, report.Findings, 1)
	finding := report.Findings[0]
	assert.Equal(t, "{99999999-8888-7777-6666-555555555555}", finding.CLSID)
	assert.Equal(t, "Example.CrashControl", finding.ClassName)
	assert.Equal(t, "Access violation", finding.ErrorMessage)
	assert.Equal(t, "0x41414141", finding.Address)
	assert.Equal(t, "write", finding.AccessType)
	assert.Contains(t, finding.Method, "Trigger")
	assert.Equal(t, 1, report.Summary.Failed)
}

// TestParseSanitizesInvalidUTF8 mirrors real reports, which dranzer writes in the
// system ANSI code page rather than UTF-8.
func TestParseSanitizesInvalidUTF8(t *testing.T) {
	// 0xae is the Latin-1 encoding of "®" and is not valid UTF-8 on its own.
	input := []byte("Number of COM Objects                   1\nProduct Name        : Example\xae Suite\n")

	report, err := Parse(input)
	require.NoError(t, err)
	assert.True(t, report.LooksLikeDranzer())
	assert.True(t, utf8.ValidString(report.Raw), "raw must be sanitized to valid UTF-8")
	assert.Equal(t, 1, report.Summary.ObjectCount)
}

func TestReportJSON(t *testing.T) {
	data, err := os.ReadFile("testdata/dranzer-report.txt")
	require.NoError(t, err)

	report, err := Parse(data)
	require.NoError(t, err)

	out, err := report.JSON()
	require.NoError(t, err)
	assert.Contains(t, string(out), `"tool"`)
	assert.Contains(t, string(out), `"summary"`)
	assert.Contains(t, string(out), `"findings"`)
}
