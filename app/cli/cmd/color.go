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

package cmd

import (
	"os"

	"golang.org/x/term"
)

// colorCapableCISignals are environment variables set by CI systems whose logs
// render ANSI escape codes, so colored output is desirable there even though
// the process is not attached to an interactive terminal. The list mirrors the
// one used by the widely adopted supports-color/chalk detection.
//
// Jenkins is intentionally absent: its console does not interpret ANSI escape
// codes unless the AnsiColor plugin is installed, so leaving it out makes the
// CLI fall back to the non-terminal default (no color) and avoids leaking raw
// escape sequences into the build log.
var colorCapableCISignals = []string{
	"GITHUB_ACTIONS",
	"GITLAB_CI",
	"CIRCLECI",
	"TRAVIS",
	"BUILDKITE",
	"DRONE",
	"APPVEYOR",
}

// LogColorDisabled reports whether colorized log output should be disabled. It
// follows the same approach as the supports-color/chalk family: honor the
// NO_COLOR / FORCE_COLOR conventions, then enable color on an interactive
// terminal or in a CI system known to render ANSI escape codes.
func LogColorDisabled() bool {
	return !colorSupported(os.LookupEnv, term.IsTerminal(int(os.Stderr.Fd())))
}

// colorSupported decides whether ANSI color should be emitted, given a way to
// look up environment variables and whether stderr is an interactive terminal.
// It is kept pure so the decision logic can be unit tested.
//
// Precedence:
//  1. NO_COLOR set, any value -> no color (https://no-color.org/). Universal
//     opt-out, wins over everything.
//  2. CLICOLOR_FORCE / FORCE_COLOR set to a non-empty, non-"0" value -> color.
//     Universal opt-in (https://bixense.com/clicolors/ and FORCE_COLOR).
//  3. TERM=dumb -> no color.
//  4. Interactive terminal -> color.
//  5. Non-terminal (CI, pipe, file) -> color only for CI systems known to
//     render ANSI (see colorCapableCISignals); otherwise no color. This keeps
//     piped output and ANSI-incapable consoles (e.g. Jenkins) clean.
func colorSupported(lookupEnv func(string) (string, bool), isTerminal bool) bool {
	if _, ok := lookupEnv("NO_COLOR"); ok {
		return false
	}

	for _, key := range []string{"CLICOLOR_FORCE", "FORCE_COLOR"} {
		if v, ok := lookupEnv(key); ok && v != "" && v != "0" {
			return true
		}
	}

	if v, ok := lookupEnv("TERM"); ok && v == "dumb" {
		return false
	}

	if isTerminal {
		return true
	}

	for _, key := range colorCapableCISignals {
		if _, ok := lookupEnv(key); ok {
			return true
		}
	}

	return false
}
