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

package buildlog

// The projection feeds a compliance gate, so what it cannot read must show:
// these tests guard the facts a policy needs to fail rather than pass a log
// whose warnings were not all read.

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRecordsParsedCounts(t *testing.T) {
	// Parsed counts occurrences, like the totals a log states about itself,
	// so a policy can compare the two.
	for name, want := range map[string]Counts{
		sampleBinlog: {Warnings: 4},
		msbuildText:  {Warnings: 4},
		vcbuildHTML:  {Warnings: 5},
	} {
		t.Run(name, func(t *testing.T) {
			report, err := Parse(readTestdata(t, name))
			require.NoError(t, err)
			assert.Equal(t, want, report.Logs[0].Parsed)
		})
	}
}

func TestParseRecordsBuildResult(t *testing.T) {
	for name, want := range map[string]string{
		sampleBinlog: ResultSucceeded,
		msbuildText:  ResultSucceeded,
		vcbuildHTML:  ResultSucceeded,
	} {
		t.Run(name, func(t *testing.T) {
			report, err := Parse(readTestdata(t, name))
			require.NoError(t, err)
			assert.Equal(t, want, report.Logs[0].Result)
		})
	}

	t.Run("failed msbuild run", func(t *testing.T) {
		report, err := Parse([]byte("Build started 1/2/2026 3:04:05 PM.\r\n" +
			"  1>C:\\src\\io.cpp(41,7): error C2065: 'buf': undeclared identifier [C:\\src\\lib.vcxproj]\r\n" +
			"Build FAILED.\r\n    0 Warning(s)\r\n    1 Error(s)\r\n"))
		require.NoError(t, err)
		assert.Equal(t, ResultFailed, report.Logs[0].Result)
	})

	t.Run("text log cut short", func(t *testing.T) {
		report, err := Parse([]byte("Build started 1/2/2026 3:04:05 PM.\r\n" +
			"  1>C:\\src\\io.cpp(41,7): warning C4996: 'strcpy': unsafe [C:\\src\\lib.vcxproj]\r\n"))
		require.NoError(t, err)
		assert.Empty(t, report.Logs[0].Result)
	})

	t.Run("failed visual c++ build", func(t *testing.T) {
		htm := bytes.ReplaceAll(readTestdata(t, vcbuildHTML), utf16le("0 error(s)"), utf16le("2 error(s)"))
		report, err := Parse(htm)
		require.NoError(t, err)
		assert.Equal(t, ResultFailed, report.Logs[0].Result)
	})
}

func utf16le(s string) []byte {
	out := make([]byte, 0, 2*len(s))
	for _, c := range []byte(s) {
		out = append(out, c, 0)
	}
	return out
}

func TestParseMSBuildRunsWithoutBuildStarted(t *testing.T) {
	// At minimal verbosity MSBuild writes no "Build started" line. Each run
	// still ends with its counts, and starts with a banner unless -nologo.
	run := func(banner bool, warning string) string {
		s := ""
		if banner {
			s += "MSBuild version 17.9.8+b34f75857 for .NET Framework\r\n"
		}
		return s + "  " + warning + "\r\n" +
			"Build succeeded.\r\n\r\n         " + warning + "\r\n\r\n    1 Warning(s)\r\n    0 Error(s)\r\n\r\nTime Elapsed 00:00:01.00\r\n"
	}
	first := "C:\\src\\a.cpp(1,1): warning C4244: conversion [C:\\src\\a.vcxproj]"
	second := "C:\\src\\b.cpp(2,2): warning C4996: 'strcpy': unsafe [C:\\src\\b.vcxproj]"

	for name, log := range map[string]string{
		"each run with a banner":        run(true, first) + run(true, second),
		"second run without its banner": "Build started 1/2/2026 3:04:05 PM.\r\n" + run(false, first) + run(false, second),
		"first run with a banner only":  run(true, first) + run(false, second),
	} {
		t.Run(name, func(t *testing.T) {
			report, err := Parse([]byte(log))
			require.NoError(t, err)
			require.Len(t, report.Logs, 2)
			assert.Equal(t, map[string]int{codeConversion: 1, codeUnsafeCRT: 1}, report.Summary.ByCode)
			for _, l := range report.Logs {
				assert.Equal(t, &Counts{Warnings: 1}, l.Reported)
				assert.Equal(t, Counts{Warnings: 1}, l.Parsed)
			}
		})
	}
}

func TestParseArchiveRecordsSkippedEntries(t *testing.T) {
	report, err := Parse(zipOf(t, map[string][]byte{
		"logs/sample.binlog": readTestdata(t, sampleBinlog),
		readmeEntry:          []byte("build logs"),
		"logs/broken.binlog": []byte("not gzip"),
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{readmeEntry, "logs/broken.binlog"}, report.Skipped)
}

func TestParseArchiveSkipsProjectImports(t *testing.T) {
	// -bl:ProjectImports=ZipFile writes the project files next to each binary
	// log as <name>.ProjectImports.zip: never logs, at any depth.
	binlogs := zipOf(t, map[string][]byte{
		"sample.binlog":             readTestdata(t, sampleBinlog),
		"sample.ProjectImports.zip": zipOf(t, map[string][]byte{csharpProject: []byte("<Project/>")}),
	})
	report, err := Parse(zipOf(t, map[string][]byte{"release/binlogs.zip": binlogs}))
	require.NoError(t, err)
	require.Len(t, report.Logs, 1)
	assert.Equal(t, []string{"release/binlogs.zip/sample.ProjectImports.zip"}, report.Skipped)
}

func TestParseBinlogSkipsLargeRecordsItDoesNotRead(t *testing.T) {
	// A large embedded project-imports archive is skipped, not read, so its
	// size does not matter.
	w := newBinlogWriter(25, 18)
	const recordProjectImportArchive = 17
	putVarint(&w.out, recordProjectImportArchive)
	putVarint(&w.out, maxBinlogRecord+1)
	w.out.Write(make([]byte, maxBinlogRecord+1))
	w.diagnostic(diagnosticRecord{kind: recordWarning, message: "m", code: codeUnusedLocal, file: "a.cs", project: csharpProject, line: 1, column: 1})

	report, err := Parse(w.bytes(t))
	require.NoError(t, err)
	assert.Equal(t, 1, report.Summary.Warnings)
}

func TestParseDiagnosticLineEdgeCases(t *testing.T) {
	testCases := []struct {
		line string
		want Diagnostic
	}{
		{
			// A column range.
			line: `C:\src\a.cpp(10,5-12): warning C4996: 'strcpy': unsafe [C:\src\a.vcxproj]`,
			want: Diagnostic{Severity: SeverityWarning, Code: codeUnsafeCRT, Message: "'strcpy': unsafe", File: `C:\src\a.cpp`, Line: 10, Column: 5, Project: `C:\src\a.vcxproj`},
		},
		{
			// clang-cl names the warning flag in brackets; it is not a project.
			line: `C:\src\a.cpp(3,1): warning : unused variable 'x' [-Wunused-variable]`,
			want: Diagnostic{Severity: SeverityWarning, Message: "unused variable 'x' [-Wunused-variable]", File: `C:\src\a.cpp`, Line: 3, Column: 1},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.line, func(t *testing.T) {
			got, ok := parseDiagnosticLine(tc.line)
			require.True(t, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSniffOldMSBuildBanner(t *testing.T) {
	assert.Equal(t, FormatMSBuildText, sniffText([]byte("Microsoft (R) Build Engine Version 3.5.30729.1\r\n[Microsoft .NET Framework, Version 2.0.50727.8806]\r\n")))
}
