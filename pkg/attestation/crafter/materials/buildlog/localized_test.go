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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A localized Visual Studio translates MSBuild's own lines (build started,
// result, counts, project) but the compiler diagnostics keep the canonical
// English "warning CODE:" form.
func TestParseLocalizedMSBuildTextLogs(t *testing.T) {
	const warning = `C:\src\a.cpp(5,3): warning C4996: 'strcpy': unsafe [C:\src\a.vcxproj]`
	testCases := []struct {
		lang, banner, start, project, result, warnings, errors string
		wantResult                                             string
		wantReported                                           *Counts
	}{
		{
			lang:   "zh-CN",
			banner: "适用于 .NET Framework MSBuild 版本 17.14.40+3e7442088", start: "生成启动时间为 2026/09/20 11:38:29。",
			project: `     1>项目“C:\src\a.sln”在节点 1 上(默认目标)。`,
			result:  "已成功生成。", warnings: "    1 个警告", errors: "    0 个错误",
			wantResult: ResultSucceeded, wantReported: &Counts{Warnings: 1},
		},
		{
			lang:  "zh-CN failed",
			start: "生成启动时间为 2018/3/13 星期二 20:57:50。", project: `1>项目“C:\src\a.sln”在节点 2 上(Build 个目标)。`,
			result: "生成失败。", warnings: "    1 个警告", errors: "    1 个错误",
			wantResult: ResultFailed, wantReported: &Counts{Warnings: 1, Errors: 1},
		},
		{
			lang:  "zh-TW",
			start: "已經開始建置於 2014/10/18 下午 02:46:20。", project: `1>節點 2 (Build 目標) 上的專案 "C:\src\a.sln"。`,
			result:     "建置成功。",
			wantResult: ResultSucceeded,
		},
		{
			lang:  "ja",
			start: "2020/05/16 15:14:01 にビルドを開始しました。", project: `1>ノード 2 上のプロジェクト "C:\src\a.sln" (Build ターゲット)。`,
			result: "ビルドに成功しました。", warnings: "    1 個の警告", errors: "    0 エラー",
			wantResult: ResultSucceeded, wantReported: &Counts{Warnings: 1},
		},
		{
			lang:  "ko",
			start: "빌드 시작: 2017-10-12 오전 2:11:06", project: `1>2 노드의 "C:\src\a.sln" 프로젝트(Build 대상)입니다.`,
			result:     "빌드하지 못했습니다.",
			wantResult: ResultFailed,
		},
		{
			lang:  "de",
			start: "Der Buildvorgang wurde am 26.01.2015 01:09:39 gestartet.", project: `1>Projekt "C:\src\a.sln" auf Knoten "2", Rebuild Ziel(e).`,
			result:     "Build erfolgreich.",
			wantResult: ResultSucceeded,
		},
		{
			lang:  "fr",
			start: "La génération a démarré 09/04/2015 11:37:23.", project: `1>Projet "C:\src\a.sln" sur le nœud 2 (Build cible(s)).`,
			result: "ÉCHEC de la build.", warnings: "    1 Avertissement(s)", errors: "    1 Erreur(s)",
			wantResult: ResultFailed, wantReported: &Counts{Warnings: 1, Errors: 1},
		},
		{
			lang:  "ru",
			start: "Сборка начата 04.04.2015 16:03:05.", project: `1>Проект "C:\src\a.sln" в узле 5 (целевые объекты Build).`,
			result: "Сборка успешно завершена.", warnings: "Предупреждений: 1", errors: "Ошибок: 0",
			wantResult: ResultSucceeded, wantReported: &Counts{Warnings: 1},
		},
		// The current translations in MSBuild's resources.
		{
			lang:  "de current",
			start: "Der Buildvorgang wurde am 28.10.2026 09:00:00 gestartet.", project: `1>Projekt "C:\src\a.sln" auf Knoten "1" (Standardziele).`,
			result: "Der Buildvorgang wurde erfolgreich ausgeführt.", warnings: "    1 Warnung(en)", errors: "    0 Fehler",
			wantResult: ResultSucceeded, wantReported: &Counts{Warnings: 1},
		},
		{
			lang:  "de failed",
			start: "Der Buildvorgang wurde am 28.10.2026 09:00:00 gestartet.", project: `1>Projekt "C:\src\a.sln" auf Knoten "1" (Standardziele).`,
			result: "Fehler beim Buildvorgang.", warnings: "    1 Warnung(en)", errors: "    1 Fehler",
			wantResult: ResultFailed, wantReported: &Counts{Warnings: 1, Errors: 1},
		},
		{
			lang:  "ko succeeded",
			start: "빌드 시작: 2026-10-28 오전 9:00:00", project: `1>1 노드의 "C:\src\a.sln" 프로젝트(기본 대상)입니다.`,
			result: "빌드했습니다.", warnings: "    경고 1개", errors: "    오류 0개",
			wantResult: ResultSucceeded, wantReported: &Counts{Warnings: 1},
		},
		{
			lang:  "ko failed with counts",
			start: "빌드 시작: 2026-10-28 오전 9:00:00", project: `1>1 노드의 "C:\src\a.sln" 프로젝트(기본 대상)입니다.`,
			result: "빌드하지 못했습니다.", warnings: "    경고 1개", errors: "    오류 1개",
			wantResult: ResultFailed, wantReported: &Counts{Warnings: 1, Errors: 1},
		},
		{
			lang:  "zh-TW failed",
			start: "已經開始建置於 2026/10/28 上午 09:00:00。", project: `1>節點 1 (預設目標) 上的專案 "C:\src\a.sln"。`,
			result: "建置失敗。", warnings: "    1 個警告", errors: "    1 個錯誤",
			wantResult: ResultFailed, wantReported: &Counts{Warnings: 1, Errors: 1},
		},
		{
			lang:  "ja failed",
			start: "2026/10/28 9:00:00 にビルドを開始しました。", project: `1>ノード 1 上のプロジェクト "C:\src\a.sln" (既定のターゲット)。`,
			result: "ビルドに失敗しました。", warnings: "    1 個の警告", errors: "    1 エラー",
			wantResult: ResultFailed, wantReported: &Counts{Warnings: 1, Errors: 1},
		},
		{
			lang:  "fr succeeded",
			start: "La génération a démarré 28/10/2026 09:00:00.", project: `1>Projet "C:\src\a.sln" sur le nœud 1 (cibles par défaut).`,
			result: "La génération a réussi.", warnings: "    1 Avertissement(s)", errors: "    0 Erreur(s)",
			wantResult: ResultSucceeded, wantReported: &Counts{Warnings: 1},
		},
		{
			lang:  "ru failed",
			start: "Сборка начата 28.10.2026 9:00:00.", project: `1>Проект "C:\src\a.sln" в узле 1 (целевые объекты по умолчанию).`,
			result: "Ошибка сборки.", warnings: "Предупреждений: 1", errors: "Ошибок: 1",
			wantResult: ResultFailed, wantReported: &Counts{Warnings: 1, Errors: 1},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.lang, func(t *testing.T) {
			lines := []string{}
			if tc.banner != "" {
				lines = append(lines, tc.banner)
			}
			lines = append(lines, tc.start, tc.project, "  1>"+warning, tc.result)
			if tc.warnings != "" {
				lines = append(lines, "", "         "+warning, "", tc.warnings, tc.errors)
			}
			report, err := Parse([]byte("\ufeff" + strings.Join(lines, "\r\n") + "\r\n"))
			require.NoError(t, err)

			require.Len(t, report.Logs, 1)
			l := report.Logs[0]
			assert.Equal(t, FormatMSBuildText, l.Format)
			assert.Equal(t, `C:\src\a.sln`, l.Solution)
			assert.Equal(t, tc.wantResult, l.Result)
			assert.Equal(t, tc.wantReported, l.Reported)
			assert.Equal(t, Counts{Warnings: 1}, l.Parsed, "the summary repeats the warning: it is read once")
			assert.Equal(t, map[string]int{codeUnsafeCRT: 1}, report.Summary.ByCode)
		})
	}
}

func TestParseCIPrefixedMSBuildLogs(t *testing.T) {
	// CI systems prefix every console line, and the build starts after the
	// output of earlier steps.
	steps := strings.Repeat("##[group]Run some earlier step with a long output line to push MSBuild past the first kilobytes\n", 200)
	body := []string{
		"MSBuild version 17.14.60+43b635718 for .NET Framework",
		"Build started 10/20/2026 9:31:59 PM.",
		`     1>Project "C:\src\a.sln" on node 1 (default targets).`,
		`     1>C:\src\a.cpp(5,3): warning C4996: 'strcpy': unsafe [C:\src\a.vcxproj]`,
		"Build succeeded.",
		`         C:\src\a.cpp(5,3): warning C4996: 'strcpy': unsafe [C:\src\a.vcxproj]`,
		"    1 Warning(s)",
		"    0 Error(s)",
	}
	for name, prefix := range map[string]string{
		"GitHub Actions or Azure DevOps": "2026-10-20T21:31:58.4328243Z ",
		"AppVeyor":                       "[00:01:12] ",
	} {
		t.Run(name, func(t *testing.T) {
			var b strings.Builder
			for _, l := range strings.Split(strings.TrimSuffix(steps, "\n"), "\n") {
				b.WriteString(prefix + l + "\n")
			}
			for _, l := range body {
				b.WriteString(prefix + l + "\n")
			}

			report, err := Parse([]byte(b.String()))
			require.NoError(t, err)
			require.Len(t, report.Logs, 1)
			l := report.Logs[0]
			assert.Equal(t, `C:\src\a.sln`, l.Solution)
			assert.Equal(t, ResultSucceeded, l.Result)
			assert.Equal(t, &Counts{Warnings: 1}, l.Reported)
			assert.Equal(t, Counts{Warnings: 1}, l.Parsed)
			assert.Equal(t, `C:\src\a.cpp`, report.Diagnostics[0].File)
		})
	}
}

func TestParseDotnetBuildConsoleLog(t *testing.T) {
	// `dotnet build` prints neither a banner nor "Build started": its result
	// line is what identifies the log.
	for lang, log := range map[string]string{
		"en": "  Determining projects to restore...\n" +
			`/src/App/Program.cs(3,5): warning CS0168: The variable 'unused' is declared but never used [/src/App/App.csproj]` + "\n" +
			"Build succeeded.\n" + `/src/App/Program.cs(3,5): warning CS0168: The variable 'unused' is declared but never used [/src/App/App.csproj]` + "\n" +
			"    1 Warning(s)\n    0 Error(s)\n",
		"ru": `/src/App/Program.cs(3,5): warning CS0168: The variable 'unused' is declared but never used [/src/App/App.csproj]` + "\n" +
			"Сборка успешно завершена.\n" + `/src/App/Program.cs(3,5): warning CS0168: The variable 'unused' is declared but never used [/src/App/App.csproj]` + "\n" +
			"Предупреждений: 1\nОшибок: 0\n",
	} {
		t.Run(lang, func(t *testing.T) {
			report, err := Parse([]byte(log))
			require.NoError(t, err)
			require.Len(t, report.Logs, 1)
			assert.Equal(t, ResultSucceeded, report.Logs[0].Result)
			assert.Equal(t, &Counts{Warnings: 1}, report.Logs[0].Reported)
			assert.Equal(t, Counts{Warnings: 1}, report.Logs[0].Parsed)
		})
	}
}
