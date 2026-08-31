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
	"io"
	"os"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
)

// hookLogFile holds the trace log file while hook commands run, so it can be
// closed by the cleanup closure returned from InitHookLogger.
var hookLogFile *os.File

// stderrMinLevel is the minimum zerolog.Level that hook commands write to
// stderr. All levels still reach the on-disk trace log; stderr is filtered
// to keep git/agent hooks quiet during normal operations.
var stderrMinLevel = zerolog.WarnLevel

// levelFilterWriter only forwards log entries at or above minLevel to w.
// Used by hook commands to keep stderr quiet while letting the trace log
// file receive every level.
type levelFilterWriter struct {
	w        io.Writer
	minLevel zerolog.Level
}

func (lw *levelFilterWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (lw *levelFilterWriter) WriteLevel(level zerolog.Level, p []byte) (n int, err error) {
	if level < lw.minLevel {
		return len(p), nil
	}

	return lw.w.Write(p)
}

// hookStderrWriter builds the colorless stderr writer used by hook commands.
// The level filter keeps stderr quiet during normal operations while still
// letting the trace log file capture every level.
func hookStderrWriter() *levelFilterWriter {
	return &levelFilterWriter{
		w: zerolog.ConsoleWriter{
			Out:             os.Stderr,
			FormatTimestamp: func(any) string { return "" },
			NoColor:         true,
		},
		minLevel: stderrMinLevel,
	}
}

// InitHookLogger reconfigures the root logger for git/agent hook commands:
// colorless output, all levels written to the trace state's log.txt, and
// Warn+ (or Debug, with --debug) written to stderr. Colorless matters because
// the hook's output is interleaved with git's during a commit or push, and
// main reads the same logger for its final error line.
//
// If trace state cannot be located or the file cannot be opened, the logger
// falls back to colorless stderr-only output so the rest of the hook still
// runs with consistent formatting. Returns a cleanup function that closes the
// log file — callers should defer it at the top of a hook's RunE.
func InitHookLogger() func() {
	closeHookLogFile()

	// --debug lowers the stderr floor; the log file always gets every level.
	if flagDebug {
		stderrMinLevel = zerolog.DebugLevel
	}

	logger = zerolog.New(hookStderrWriter()).Level(stderrMinLevel)

	store, _, err := state.Locate()
	if err != nil {
		return func() {}
	}

	if err := initHookLogFile(store); err != nil {
		logger.Debug().Err(err).Msg("could not open hook log file")
		return func() {}
	}

	return closeHookLogFile
}

// initHookLogFile opens the trace log file and reassigns the root logger to a
// MultiLevelWriter that writes all levels to the file and delegates stderr
// output to a level-filtered colorless console writer.
func initHookLogFile(store *state.Store) error {
	logPath := store.LogFilePath()

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	hookLogFile = f

	fileWriter := zerolog.ConsoleWriter{
		Out:     f,
		NoColor: true,
	}

	logger = zerolog.New(zerolog.MultiLevelWriter(fileWriter, hookStderrWriter())).
		Level(zerolog.DebugLevel).
		With().Timestamp().Logger()

	return nil
}

func closeHookLogFile() {
	if hookLogFile != nil {
		_ = hookLogFile.Close()
		hookLogFile = nil
	}
}
