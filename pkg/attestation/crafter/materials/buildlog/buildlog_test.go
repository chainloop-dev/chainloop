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

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testdataDir   = "../testdata/buildlog/"
	sampleBinlog  = "sample.binlog"
	msbuildText   = "msbuild.txt"
	vcbuildHTML   = "vcbuild.htm"
	readmeEntry   = "logs/README.txt"
	csharpProject = "a.csproj"
)

// Codes and names shared by several assertions.
const (
	codeBrokenCrypto = "CA5351"
	codeConversion   = "C4244"
	codeUnsafeCRT    = "C4996"
	codeUnusedLocal  = "CS0168"
	codeNullConvert  = "CS8600"
	codeNullDeref    = "CS8602"
	codeUndeclared   = "C2065"
	msgUndeclared    = "'buf': undeclared identifier"
	appProject       = "/src/App/App.csproj"
	appSource        = "/src/App/Program.cs"
	vcProject        = "demo"
)

// sampleDiagnostics are the warnings the synthetic two-project solution in
// testdata produces, whichever log format recorded the build.
var sampleDiagnostics = []Diagnostic{
	{Severity: SeverityWarning, Code: codeBrokenCrypto, Message: "Weak uses a broken cryptographic algorithm MD5 (https://learn.microsoft.com/dotnet/fundamentals/code-analysis/quality-rules/ca5351)", File: "/src/Lib/Hashing.cs", Line: 9, Column: 25, Project: "/src/Lib/Lib.csproj", Occurrences: 1},
	{Severity: SeverityWarning, Code: codeUnusedLocal, Message: "The variable 'unused' is declared but never used", File: appSource, Line: 3, Column: 5, Project: appProject, Occurrences: 1},
	{Severity: SeverityWarning, Code: codeNullConvert, Message: "Converting null literal or possible null value to non-nullable type.", File: appSource, Line: 4, Column: 12, Project: appProject, Occurrences: 1},
	{Severity: SeverityWarning, Code: codeNullDeref, Message: "Dereference of a possibly null reference.", File: appSource, Line: 5, Column: 19, Project: appProject, Occurrences: 1},
}

var sampleSummary = Summary{
	Warnings: 4,
	ByCode:   map[string]int{codeBrokenCrypto: 1, codeUnusedLocal: 1, codeNullConvert: 1, codeNullDeref: 1},
}

// inLogs returns diagnostics as seen in the given logs of a report.
func inLogs(ds []Diagnostic, logs ...int) []Diagnostic {
	out := make([]Diagnostic, len(ds))
	for i, d := range ds {
		d.Logs = logs
		out[i] = d
	}
	return out
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(testdataDir + name)
	require.NoError(t, err)
	return data
}

func zipOf(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestParseMSBuildBinlog(t *testing.T) {
	report, err := Parse(readTestdata(t, sampleBinlog))
	require.NoError(t, err)

	assert.Equal(t, Tool{Name: ToolMSBuild}, report.Tool)
	require.Len(t, report.Logs, 1)
	assert.Equal(t, FormatMSBuildBinlog, report.Logs[0].Format)
	// The build's root project: the solution the binary log was produced for.
	assert.Equal(t, "/src/Sample.sln", report.Logs[0].Solution)
	assert.Equal(t, sampleSummary, report.Summary)
	assert.Equal(t, inLogs(sampleDiagnostics, 0), report.Diagnostics)
}

func TestParseMSBuildTextLog(t *testing.T) {
	report, err := Parse(readTestdata(t, msbuildText))
	require.NoError(t, err)

	require.Len(t, report.Logs, 1)
	assert.Equal(t, FormatMSBuildText, report.Logs[0].Format)
	assert.Equal(t, "/src/Sample.sln", report.Logs[0].Solution)
	// The warnings MSBuild repeats in its end-of-build summary must not be
	// counted twice.
	assert.Equal(t, sampleSummary, report.Summary)
	assert.Equal(t, inLogs(sampleDiagnostics, 0), report.Diagnostics)
	assert.Equal(t, &Counts{Warnings: 4}, report.Logs[0].Reported)
}

func TestParseMSBuildTextLogOfSeveralSolutions(t *testing.T) {
	// One text log can hold several MSBuild invocations, one per solution,
	// appended to each other. Each repeats its warnings in its own summary;
	// a project both solutions build reports its warning once per solution.
	invocation := func(solution string, warnings ...string) string {
		s := "MSBuild version 17.9.8+b34f75857 for .NET Framework\r\n" +
			"Build started 1/2/2026 3:04:05 PM.\r\n" +
			"Project \"" + solution + "\" on node 1 (default targets).\r\n"
		for _, w := range warnings {
			s += "  1>" + w + "\r\n"
		}
		s += "Build succeeded.\r\n\r\n"
		for _, w := range warnings {
			s += "         " + w + "\r\n"
		}
		return s + fmt.Sprintf("\r\n    %d Warning(s)\r\n    0 Error(s)\r\n\r\n", len(warnings))
	}
	shared := "/src/Lib/Hashing.cs(9,25): warning CA5351: Weak uses a broken cryptographic algorithm MD5 (https://learn.microsoft.com/dotnet/fundamentals/code-analysis/quality-rules/ca5351) [/src/Lib/Lib.csproj]"
	app := "/src/App/Program.cs(3,5): warning CS0168: The variable 'unused' is declared but never used [/src/App/App.csproj]"
	log := invocation("/src/A.sln", shared, app) + invocation("/src/B.sln", shared)

	report, err := Parse([]byte(log))
	require.NoError(t, err)

	// Each MSBuild run is a log of its own, so the warnings stay attributed to
	// the solution that produced them.
	require.Len(t, report.Logs, 2)
	assert.Equal(t, "/src/A.sln", report.Logs[0].Solution)
	assert.Equal(t, &Counts{Warnings: 2}, report.Logs[0].Reported)
	assert.Equal(t, "/src/B.sln", report.Logs[1].Solution)
	assert.Equal(t, &Counts{Warnings: 1}, report.Logs[1].Reported)
	assert.Equal(t, report.Logs[0].Source, report.Logs[1].Source)

	assert.Equal(t, Summary{Warnings: 2, ByCode: map[string]int{codeBrokenCrypto: 1, codeUnusedLocal: 1}}, report.Summary)
	want := []Diagnostic{sampleDiagnostics[0], sampleDiagnostics[1]}
	want[0].Occurrences = 2
	want[0].Logs = []int{0, 1}
	want[1].Logs = []int{0}
	assert.Equal(t, want, report.Diagnostics)
}

func TestParseArchiveOfVCBuildLogs(t *testing.T) {
	// Visual C++ writes one BuildLog.htm per project, so a solution's logs come
	// zipped. Each entry is a log of its own.
	htm := readTestdata(t, vcbuildHTML)
	report, err := Parse(zipOf(t, map[string][]byte{
		"demo/Release/BuildLog.htm":  htm,
		"other/Release/BuildLog.htm": htm,
	}))
	require.NoError(t, err)

	require.Len(t, report.Logs, 2)
	for i, source := range []string{"demo/Release/BuildLog.htm", "other/Release/BuildLog.htm"} {
		assert.Equal(t, source, report.Logs[i].Source)
		assert.Equal(t, FormatVCBuildHTML, report.Logs[i].Format)
		assert.Equal(t, &Counts{Warnings: 5}, report.Logs[i].Reported)
	}
	assert.Equal(t, Tool{Name: ToolVCBuild}, report.Tool)
	assert.Equal(t, 4, report.Summary.Warnings)
	for _, d := range report.Diagnostics {
		assert.Equal(t, vcProject, d.Project)
		assert.Equal(t, []int{0, 1}, d.Logs)
	}
}

func TestParseMSBuildTextLogSummaryOnly(t *testing.T) {
	// A console log captured with -clp:Summary only carries the warnings in
	// the end-of-build summary; they must still be picked up.
	log := "Build started 1/2/2026 3:04:05 PM.\r\n" +
		"Build succeeded.\r\n" +
		"\r\n" +
		"       \"/src/App/App.csproj\" (default target) (1) ->\r\n" +
		"         /src/App/Program.cs(3,5): warning CS0168: The variable 'unused' is declared but never used [/src/App/App.csproj]\r\n" +
		"\r\n" +
		"    1 Warning(s)\r\n" +
		"    0 Error(s)\r\n"

	report, err := Parse([]byte(log))
	require.NoError(t, err)
	assert.Equal(t, inLogs([]Diagnostic{sampleDiagnostics[1]}, 0), report.Diagnostics)
}

func TestParseMSBuildTextLogWithBanner(t *testing.T) {
	log := "MSBuild version 17.9.8+b34f75857 for .NET Framework\r\n" +
		"Build started 1/2/2026 3:04:05 PM.\r\n" +
		"  1>C:\\src\\lib\\io.cpp(41,7): error C2065: 'buf': undeclared identifier [C:\\src\\lib\\lib.vcxproj]\r\n" +
		"Build FAILED.\r\n" +
		"    0 Warning(s)\r\n" +
		"    1 Error(s)\r\n"

	report, err := Parse([]byte(log))
	require.NoError(t, err)

	assert.Equal(t, Tool{Name: ToolMSBuild, Version: "17.9.8+b34f75857"}, report.Tool)
	assert.Equal(t, Summary{Errors: 1, ByCode: map[string]int{codeUndeclared: 1}}, report.Summary)
	assert.Equal(t, []Diagnostic{{
		Severity: SeverityError, Code: codeUndeclared, Message: msgUndeclared,
		File: "C:\\src\\lib\\io.cpp", Line: 41, Column: 7, Project: "C:\\src\\lib\\lib.vcxproj", Occurrences: 1, Logs: []int{0},
	}}, report.Diagnostics)
	assert.Equal(t, &Counts{Errors: 1}, report.Logs[0].Reported)
}

func TestParseVCBuildHTMLLog(t *testing.T) {
	report, err := Parse(readTestdata(t, vcbuildHTML))
	require.NoError(t, err)

	assert.Equal(t, Tool{Name: ToolVCBuild}, report.Tool)
	require.Len(t, report.Logs, 1)
	assert.Equal(t, FormatVCBuildHTML, report.Logs[0].Format)
	assert.Equal(t, &Counts{Warnings: 5}, report.Logs[0].Reported)

	assert.Equal(t, Summary{
		Warnings: 4,
		ByCode:   map[string]int{codeConversion: 1, "C4706": 1, codeUnsafeCRT: 1, "LNK4099": 1},
	}, report.Summary)
	assert.Equal(t, inLogs([]Diagnostic{
		// The header warning is reported once per translation unit including it,
		// with the path spelled in different case; it is one diagnostic.
		{Severity: SeverityWarning, Code: codeConversion, Message: "'initializing' : conversion from 'int' to 'char', possible loss of data", File: "C:\\SDK\\include\\vendor.h", Line: 7, Project: vcProject, Occurrences: 2},
		{Severity: SeverityWarning, Code: "C4706", Message: "assignment within conditional expression", File: ".\\util.cpp", Line: 30, Project: vcProject, Occurrences: 1},
		{Severity: SeverityWarning, Code: codeUnsafeCRT, Message: "'strcpy': This function or variable may be unsafe. Consider using strcpy_s instead.", File: ".\\main.cpp", Line: 12, Project: vcProject, Occurrences: 1},
		{Severity: SeverityWarning, Code: "LNK4099", Message: "PDB 'vc90.pdb' was not found with 'vendor.lib'; linking object as if no debug info", File: "vendor.lib(io.obj)", Project: vcProject, Occurrences: 1},
	}, 0), report.Diagnostics)
}

func TestParseArchiveOfBinlogs(t *testing.T) {
	// One binlog per solution, zipped together. The library is built by both
	// solutions, so its warning is one diagnostic seen twice.
	bundle := zipOf(t, map[string][]byte{
		"logs/sample.binlog": readTestdata(t, sampleBinlog),
		"logs/lib.binlog":    readTestdata(t, "lib.binlog"),
		readmeEntry:          []byte("not a build log"),
	})

	report, err := Parse(bundle)
	require.NoError(t, err)

	require.Len(t, report.Logs, 2)
	assert.Equal(t, "logs/lib.binlog", report.Logs[0].Source)
	assert.Equal(t, "/src/Lib/Lib.csproj", report.Logs[0].Solution)
	assert.Equal(t, Summary{Warnings: 1, ByCode: map[string]int{codeBrokenCrypto: 1}}, report.Logs[0].Summary)
	assert.Equal(t, "logs/sample.binlog", report.Logs[1].Source)
	assert.Equal(t, "/src/Sample.sln", report.Logs[1].Solution)
	assert.Equal(t, sampleSummary, report.Logs[1].Summary)

	assert.Equal(t, sampleSummary, report.Summary)
	// Each diagnostic names the logs it was seen in: the library's warning both.
	want := inLogs(sampleDiagnostics, 1)
	want[0].Occurrences = 2
	want[0].Logs = []int{0, 1}
	assert.Equal(t, want, report.Diagnostics)
}

// everythingZip is one archive with every log of a release in it, in folders,
// next to files that are not logs: the binary logs zipped per solution as
// their own archive, a text log of the same build, and per-project Visual C++
// logs.
func everythingZip(t *testing.T) []byte {
	t.Helper()
	binlogs := zipOf(t, map[string][]byte{
		sampleBinlog: readTestdata(t, sampleBinlog),
		"lib.binlog": readTestdata(t, "lib.binlog"),
	})
	return zipOf(t, map[string][]byte{
		"release/binlogs.zip":                  binlogs,
		"release/msbuild/build.log":            readTestdata(t, msbuildText),
		"release/ce/demo/Release/BuildLog.htm": readTestdata(t, vcbuildHTML),
		"release/README.txt":                   []byte("build logs of the release"),
		// A package is a zip too, but not a bundle of logs.
		"release/packages/tool.nupkg": zipOf(t, map[string][]byte{"tool.nuspec": []byte("<package/>")}),
	})
}

func TestParseArchiveOfEverything(t *testing.T) {
	report, err := Parse(everythingZip(t))
	require.NoError(t, err)

	sources := make([]string, 0, len(report.Logs))
	formats := map[string]string{}
	for _, l := range report.Logs {
		sources = append(sources, l.Source)
		formats[l.Source] = l.Format
	}
	assert.Equal(t, []string{
		"release/binlogs.zip/lib.binlog",
		"release/binlogs.zip/sample.binlog",
		"release/ce/demo/Release/BuildLog.htm",
		"release/msbuild/build.log",
	}, sources)
	assert.Equal(t, FormatMSBuildBinlog, formats["release/binlogs.zip/lib.binlog"])
	assert.Equal(t, FormatVCBuildHTML, formats["release/ce/demo/Release/BuildLog.htm"])
	assert.Equal(t, FormatMSBuildText, formats["release/msbuild/build.log"])

	// The C# warnings and the Visual C++ ones, each once: the text log and the
	// binary log record the same build.
	assert.Equal(t, 8, report.Summary.Warnings)
	occurrences := map[string]int{}
	for _, d := range report.Diagnostics {
		occurrences[d.Code] += d.Occurrences
	}
	// Built by both solutions, and in the text log too.
	assert.Equal(t, 3, occurrences[codeBrokenCrypto])
	assert.Equal(t, 2, occurrences[codeUnusedLocal])
}

func TestParseFileOfEverythingMatchesParse(t *testing.T) {
	data := everythingZip(t)
	p := filepath.Join(t.TempDir(), "release.zip")
	require.NoError(t, os.WriteFile(p, data, 0o600))

	fromFile, err := ParseFile(p)
	require.NoError(t, err)
	fromBytes, err := Parse(data)
	require.NoError(t, err)
	assert.Equal(t, fromBytes, fromFile)
}

func TestParseArchiveNestedTooDeep(t *testing.T) {
	// Logs two archives down are not read. That must fail, not pass as a
	// bundle without them.
	inner := zipOf(t, map[string][]byte{sampleBinlog: readTestdata(t, sampleBinlog)})
	middle := zipOf(t, map[string][]byte{"inner.zip": inner})
	outer := zipOf(t, map[string][]byte{
		"middle.zip":       middle,
		"vcbuild/demo.htm": readTestdata(t, vcbuildHTML),
	})

	_, err := Parse(outer)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoBuildLog)
	assert.ErrorContains(t, err, "middle.zip/inner.zip")
}

func TestParseArchiveWithoutBuildLogs(t *testing.T) {
	_, err := Parse(zipOf(t, map[string][]byte{"README.txt": []byte("hello")}))
	assert.ErrorIs(t, err, ErrNoBuildLog)
}

func TestParseRejectsOtherContent(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":      nil,
		"plain text": []byte("hello world\n"),
		"json":       []byte(`{"runs": []}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(data)
			assert.ErrorIs(t, err, ErrNoBuildLog)
		})
	}
}

func TestParseFileMatchesParse(t *testing.T) {
	for _, name := range []string{sampleBinlog, msbuildText, vcbuildHTML} {
		t.Run(name, func(t *testing.T) {
			fromFile, err := ParseFile(testdataDir + name)
			require.NoError(t, err)
			fromBytes, err := Parse(readTestdata(t, name))
			require.NoError(t, err)
			assert.Equal(t, fromBytes, fromFile)
		})
	}
}

func TestReportJSONShape(t *testing.T) {
	report, err := Parse(readTestdata(t, sampleBinlog))
	require.NoError(t, err)

	raw, err := json.Marshal(report)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	assert.Equal(t, map[string]any{"name": "msbuild"}, doc["tool"])
	assert.Equal(t, map[string]any{
		"warnings": float64(4),
		"errors":   float64(0),
		"by_code":  map[string]any{codeBrokenCrypto: float64(1), codeUnusedLocal: float64(1), codeNullConvert: float64(1), codeNullDeref: float64(1)},
	}, doc["summary"])
	diagnostics := doc["diagnostics"].([]any)
	assert.Equal(t, map[string]any{
		"severity":    "warning",
		"code":        codeBrokenCrypto,
		"message":     sampleDiagnostics[0].Message,
		"file":        "/src/Lib/Hashing.cs",
		"line":        float64(9),
		"column":      float64(25),
		"project":     "/src/Lib/Lib.csproj",
		"occurrences": float64(1),
		"logs":        []any{float64(0)},
	}, diagnostics[0])
	logs := doc["logs"].([]any)
	assert.Equal(t, "/src/Sample.sln", logs[0].(map[string]any)["solution"])
}
