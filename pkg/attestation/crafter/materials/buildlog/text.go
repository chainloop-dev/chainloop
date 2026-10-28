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
	"bufio"
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var (
	// diagnosticRe matches MSBuild's canonical error format, which both
	// MSBuild and the Visual C++ tools write:
	//   origin(location) : [subcategory] category code : text [project]
	// The origin is a file, or a tool name such as "LINK" or "lib.lib(obj.obj)";
	// the location is "(line)", "(line,col)", "(line-line)", "(line,col-col)" or
	// "(line,col,line,col)". MSBuild prefixes a node id ("3>") when it builds in
	// parallel and appends the project in brackets. A bracket starting with "-"
	// is a clang-cl warning flag, not a project.
	diagnosticRe = regexp.MustCompile(`^\s*(?:\d+(?::\d+)?>)?(.*?)(?:\((\d+)(?:([,-])(\d+))?(?:[,-]\d+)*\))?\s*:\s*(?:[^:]*?\s)?(fatal error|error|warning)(?:\s+([A-Za-z]+[0-9]+))?\s*:\s*(.*?)(?:\s+\[([^\]\-][^\]]*)\])?\s*$`)
	// MSBuild's own lines are translated when Visual Studio runs in another
	// language, while compiler diagnostics keep the canonical English form. The
	// patterns below hold the English line and the translations seen in real
	// logs (zh-CN, zh-TW, ja, ko, de, fr, ru). A translation not listed leaves
	// the fact unknown, which the policy treats as incomplete evidence.

	// msbuildBannerRe extracts the MSBuild version from the console banner.
	msbuildBannerRe = regexp.MustCompile(`^(?:MSBuild (?i:version)|Microsoft \(R\) Build Engine (?i:version)|.*MSBuild 版本) ([0-9][^\s]*)`)
	// msbuildStartRe and the result patterns delimit one MSBuild invocation: the
	// diagnostics logged while it runs, then the end-of-build summary that
	// repeats them.
	msbuildStartRe     = regexp.MustCompile(`^(?:Build started\b|生成启动时间为|已經開始建置於|빌드 시작:|Der Buildvorgang wurde am .* gestartet\.|La génération a démarré|Сборка начата)|にビルドを開始しました。\s*$`)
	msbuildSucceededRe = regexp.MustCompile(`^(?:Build succeeded\.|已成功生成。|生成成功。|建置成功。|ビルドに成功しました。|Build erfolgreich\.|Сборка успешно завершена\.)\s*$`)
	msbuildFailedRe    = regexp.MustCompile(`^(?:Build FAILED\.|生成失败。|빌드하지 못했습니다\.|ÉCHEC de la build\.)\s*$`)
	// msbuildProjectRe matches the start of a project build,
	// `Project "path" on node N`, which may follow other text on its line.
	msbuildProjectRe = regexp.MustCompile(`\bProject "([^"]+)" on node \d+|项目“([^”]+)”在节点 \d+ 上|節點 \d+ .*?上的專案 "([^"]+)"|ノード \d+ 上のプロジェクト "([^"]+)"|\d+ 노드의 "([^"]+)" 프로젝트|Projekt "([^"]+)" auf Knoten|Projet "([^"]+)" sur le nœud|Проект "([^"]+)" в узле`)
	// msbuildWarningsRe and msbuildErrorsRe match the summary counters
	// "N Warning(s)" and "N Error(s)".
	msbuildWarningsRe = regexp.MustCompile(`^\s*(?:(\d+) (?:Warning\(s\)|个警告|個の警告|Avertissement\(s\))|Предупреждений: (\d+))\s*$`)
	msbuildErrorsRe   = regexp.MustCompile(`^\s*(?:(\d+) (?:Error\(s\)|个错误|エラー|Erreur\(s\))|Ошибок: (\d+))\s*$`)
	// ciPrefixRe matches what CI systems put before each console line: an ISO
	// timestamp (GitHub Actions, Azure DevOps) or "[hh:mm:ss] " (AppVeyor).
	ciPrefixRe = regexp.MustCompile(`^(?:\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z |\[\d{2}:\d{2}:\d{2}\] )`)

	// vcbuildProjectRe matches the per-project heading of a Visual C++ build log.
	vcbuildProjectRe = regexp.MustCompile(`Build started: Project: ([^,]+),`)
	// vcbuildResultRe matches the per-project result line "name - N error(s), M warning(s)".
	vcbuildResultRe = regexp.MustCompile(`^.+ - (\d+) error\(s\), (\d+) warning\(s\)`)
	// htmlTagRe matches the markup a Visual C++ build log wraps its text in.
	htmlTagRe = regexp.MustCompile(`<[^>]*>`)
)

// maxLineLength bounds one log line. Diagnostic-verbosity logs echo whole
// compiler command lines and item lists, so lines of tens of kilobytes are
// normal.
const maxLineLength = 64 << 20

// sniffText decides whether peeked bytes start a text build log, and which
// kind. It returns the empty string for anything else. An MSBuild log is
// recognized by its banner, "Build started" or result line anywhere in the
// peek: a CI console capture shows the output of earlier steps first, and
// `dotnet build` prints neither a banner nor "Build started".
func sniffText(peek []byte) string {
	head, err := decodeText(peek)
	if err != nil {
		return ""
	}
	if strings.Contains(strings.ToLower(head), "<html") && strings.Contains(head, "Build Log") {
		return FormatVCBuildHTML
	}
	for _, line := range strings.Split(head, "\n") {
		line = strings.TrimSpace(ciPrefixRe.ReplaceAllString(strings.TrimRight(line, "\r"), ""))
		if msbuildBannerRe.MatchString(line) || msbuildStartRe.MatchString(line) ||
			msbuildSucceededRe.MatchString(line) || msbuildFailedRe.MatchString(line) {
			return FormatMSBuildText
		}
	}
	return ""
}

// firstMatch returns the first non-empty submatch of m.
func firstMatch(m []string) string {
	for _, g := range m[1:] {
		if g != "" {
			return g
		}
	}
	return ""
}

// decodeText decodes a prefix of a log to UTF-8, tolerating a cut-off final
// character.
func decodeText(peek []byte) (string, error) {
	out, _, err := transform.Bytes(textDecoder(peek), peek)
	if err != nil && len(out) == 0 {
		return "", err
	}
	return string(out), nil
}

// textDecoder picks the decoder for a log from its first bytes. Visual Studio
// writes UTF-16 build logs, normally with a byte order mark; one without it is
// recognized by the zero high bytes of its ASCII text.
func textDecoder(peek []byte) transform.Transformer {
	if !bytes.HasPrefix(peek, []byte{0xff, 0xfe}) && !bytes.HasPrefix(peek, []byte{0xfe, 0xff}) && looksLikeUTF16LE(peek) {
		return unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
	}
	return unicode.BOMOverride(unicode.UTF8.NewDecoder())
}

func looksLikeUTF16LE(peek []byte) bool {
	n := min(len(peek), 64) &^ 1
	if n == 0 {
		return false
	}
	for i := 1; i < n; i += 2 {
		if peek[i] != 0 {
			return false
		}
	}
	return true
}

// run collects one MSBuild invocation of a text log.
type run struct {
	version, solution, result string
	inline, summary           []Diagnostic
	opened, inSummary         bool
	reported                  *Counts
}

// empty reports whether nothing of the run has been seen yet.
func (r *run) empty() bool {
	return !r.opened && r.result == "" && r.reported == nil && len(r.inline)+len(r.summary) == 0
}

func (r *run) count() *Counts {
	if r.reported == nil {
		r.reported = &Counts{}
	}
	return r.reported
}

// log returns the run as a parsed log. Its diagnostics are those logged while
// building, or the summary's when the log only kept the summary (-clp:Summary).
func (r *run) log() *parsedLog {
	diagnostics := r.inline
	if len(diagnostics) == 0 {
		diagnostics = r.summary
	}
	return &parsedLog{
		format:      FormatMSBuildText,
		tool:        Tool{Name: ToolMSBuild, Version: r.version},
		solution:    r.solution,
		result:      r.result,
		diagnostics: diagnostics,
		reported:    r.reported,
	}
}

// parseText reads the builds recorded in a text build log of the given format,
// streaming it line by line so that large diagnostic-verbosity logs are not
// held in memory.
func parseText(r *bufio.Reader, format string) ([]*parsedLog, error) {
	peek, _ := r.Peek(4096)
	scanner := bufio.NewScanner(transform.NewReader(r, textDecoder(peek)))
	scanner.Buffer(make([]byte, 0, 64<<10), maxLineLength)

	var logs []*parsedLog
	var err error
	if format == FormatVCBuildHTML {
		var log *parsedLog
		if log, err = scanVCBuild(scanner); log != nil {
			logs = []*parsedLog{log}
		}
	} else {
		logs, err = scanMSBuild(scanner)
	}
	if err != nil {
		return nil, fmt.Errorf("reading build log: %w", err)
	}
	return logs, nil
}

// scanVCBuild reads a Visual C++ build log: one project's build.
func scanVCBuild(scanner *bufio.Scanner) (*parsedLog, error) {
	log := &parsedLog{format: FormatVCBuildHTML, tool: Tool{Name: ToolVCBuild}}
	var project string
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if m := vcbuildProjectRe.FindStringSubmatch(line); m != nil {
			project = strings.TrimSpace(html.UnescapeString(m[1]))
			continue
		}
		line = html.UnescapeString(htmlTagRe.ReplaceAllString(line, ""))
		if m := vcbuildResultRe.FindStringSubmatch(line); m != nil {
			if log.reported == nil {
				log.reported = &Counts{}
			}
			errs, _ := strconv.Atoi(m[1])
			warns, _ := strconv.Atoi(m[2])
			log.reported.Errors += errs
			log.reported.Warnings += warns
			if log.reported.Errors > 0 {
				log.result = ResultFailed
			} else {
				log.result = ResultSucceeded
			}
			continue
		}
		if d, ok := parseDiagnosticLine(line); ok {
			if d.Project == "" {
				d.Project = project
			}
			log.diagnostics = append(log.diagnostics, d)
		}
	}
	return log, scanner.Err()
}

// scanMSBuild reads an MSBuild text log. Several invocations may be appended to
// one file (one per solution); each is returned as a log of its own, so that
// their warnings stay attributed to the solution that produced them. A run
// starts at its banner or "Build started" line and ends with the counts of its
// summary; at minimal verbosity there is no "Build started", and with -nologo
// no banner, so any of them delimits it.
func scanMSBuild(scanner *bufio.Scanner) ([]*parsedLog, error) {
	var logs []*parsedLog
	cur := &run{}
	next := func() {
		if !cur.empty() {
			logs = append(logs, cur.log())
		}
		cur = &run{}
	}
	for scanner.Scan() {
		line := ciPrefixRe.ReplaceAllString(strings.TrimRight(scanner.Text(), "\r"), "")

		switch {
		case msbuildBannerRe.MatchString(line):
			next()
			cur.version, cur.opened = msbuildBannerRe.FindStringSubmatch(line)[1], true
			continue
		case msbuildStartRe.MatchString(line):
			// Keep what came before it too: MSBuild reports some command-line
			// errors before it starts building.
			if cur.inSummary || cur.result != "" {
				next()
			}
			cur.opened = true
			continue
		case msbuildSucceededRe.MatchString(line):
			cur.inSummary, cur.result = true, ResultSucceeded
			continue
		case msbuildFailedRe.MatchString(line):
			cur.inSummary, cur.result = true, ResultFailed
			continue
		}
		if cur.solution == "" {
			// The first project a run builds is the one it was started for.
			if m := msbuildProjectRe.FindStringSubmatch(line); m != nil {
				cur.solution = firstMatch(m)
			}
		}
		if cur.inSummary {
			if m := msbuildWarningsRe.FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(firstMatch(m))
				cur.count().Warnings += n
				continue
			}
			if m := msbuildErrorsRe.FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(firstMatch(m))
				cur.count().Errors += n
				// The error count closes the run's summary.
				next()
				continue
			}
		}
		if d, ok := parseDiagnosticLine(line); ok {
			if cur.inSummary {
				cur.summary = append(cur.summary, d)
			} else {
				cur.inline = append(cur.inline, d)
			}
		}
	}
	next()
	if len(logs) == 0 {
		// A recognized log with nothing in it is still one build.
		logs = append(logs, (&run{}).log())
	}
	return logs, scanner.Err()
}

// parseDiagnosticLine parses one line in canonical error format.
func parseDiagnosticLine(line string) (Diagnostic, bool) {
	m := diagnosticRe.FindStringSubmatch(line)
	if m == nil {
		return Diagnostic{}, false
	}
	d := Diagnostic{
		File:     strings.TrimSpace(m[1]),
		Severity: SeverityWarning,
		Code:     m[6],
		Message:  m[7],
		Project:  m[8],
	}
	if m[5] != "warning" {
		d.Severity = SeverityError
	}
	d.Line, _ = strconv.Atoi(m[2])
	// "(line,col)" carries a column; "(line-line)" is a range of lines.
	if m[3] == "," {
		d.Column, _ = strconv.Atoi(m[4])
	}
	return d, true
}
