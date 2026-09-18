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

//go:build !windows

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFlushCommand points the spawn at a shell snippet instead of the real binary, and
// returns the path that snippet receives as $1 to write what it read on stdin to.
func stubFlushCommand(t *testing.T, script string) string {
	t.Helper()

	out := filepath.Join(t.TempDir(), "received")

	original := telemetryFlushCommand
	telemetryFlushCommand = func() (string, []string, error) {
		// `sh -c <script> <name> <arg>` makes the script's $1 the output path.
		return "/bin/sh", []string{"-c", script, "telemetry-flush-stub", out}, nil
	}
	t.Cleanup(func() { telemetryFlushCommand = original })

	return out
}

// waitForFile polls until the detached child has written its output, which by design
// happens after spawnTelemetryFlush has already returned.
func waitForFile(t *testing.T, path string) []byte {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if content, err := os.ReadFile(path); err == nil && len(content) > 0 {
			return content
		}
		time.Sleep(10 * time.Millisecond)
	}

	require.FailNowf(t, "the child never produced its output", "path: %s", path)
	return nil
}

// assertNoChildOutput gives a child that should never have been started long enough to
// prove it was not.
func assertNoChildOutput(t *testing.T, path, msg string) {
	t.Helper()

	time.Sleep(200 * time.Millisecond)
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), msg)
}

// TestSpawnTelemetryFlushDeliversThePayload covers the process boundary itself: what the
// parent writes is what the child reads on its stdin.
func TestSpawnTelemetryFlushDeliversThePayload(t *testing.T) {
	out := stubFlushCommand(t, `cat > "$1"`)

	payload := []byte(`{"command":"workflow list","duration_ms":1234,"success":true}`)
	require.NoError(t, spawnTelemetryFlush(payload))

	assert.JSONEq(t, string(payload), string(waitForFile(t, out)))
}

// TestSpawnTelemetryFlushDoesNotWaitForTheChild is the guarantee the whole design exists
// for: the command must not pay for delivery. The stub sleeps well past the time the
// parent is allowed to take, so a parent that waited would be caught here.
func TestSpawnTelemetryFlushDoesNotWaitForTheChild(t *testing.T) {
	const childSleep = 2 * time.Second
	out := stubFlushCommand(t, fmt.Sprintf(`cat > "$1".tmp; sleep %d; mv "$1".tmp "$1"`, int(childSleep.Seconds())))

	start := time.Now()
	require.NoError(t, spawnTelemetryFlush([]byte(`{"command":"workflow list"}`)))
	elapsed := time.Since(start)

	assert.Less(t, elapsed, childSleep/2,
		"spawnTelemetryFlush returned in %s; it must not wait on the child", elapsed)

	// And the child still finishes its work afterwards.
	assert.NotEmpty(t, waitForFile(t, out))
}

// TestSpawnTelemetryFlushSurvivesABrokenChild covers the error paths: a child that cannot
// be started, or one that exits without reading, must not hang or panic the parent.
func TestSpawnTelemetryFlushSurvivesABrokenChild(t *testing.T) {
	testCases := []struct {
		name    string
		arrange func(t *testing.T)
		wantErr bool
	}{
		{
			name: "the executable cannot be resolved",
			arrange: func(t *testing.T) {
				original := telemetryFlushCommand
				telemetryFlushCommand = func() (string, []string, error) {
					return "", nil, errors.New("no executable")
				}
				t.Cleanup(func() { telemetryFlushCommand = original })
			},
			wantErr: true,
		},
		{
			name: "the executable does not exist",
			arrange: func(t *testing.T) {
				missing := filepath.Join(t.TempDir(), "missing")
				original := telemetryFlushCommand
				telemetryFlushCommand = func() (string, []string, error) {
					return missing, nil, nil
				}
				t.Cleanup(func() { telemetryFlushCommand = original })
			},
			wantErr: true,
		},
		{
			name: "the child exits without reading stdin",
			arrange: func(t *testing.T) {
				stubFlushCommand(t, `exit 0`)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.arrange(t)

			done := make(chan error, 1)
			go func() { done <- spawnTelemetryFlush([]byte(`{"command":"workflow list"}`)) }()

			select {
			case err := <-done:
				if tc.wantErr {
					assert.Error(t, err)
				}
			case <-time.After(10 * time.Second):
				require.FailNow(t, "spawnTelemetryFlush blocked; it must never hold up the exit path")
			}
		})
	}
}

// TestReportCommandIsSilentWhenTelemetryIsDisabled checks the opt-out at the level that
// matters: no child process is started at all.
func TestReportCommandIsSilentWhenTelemetryIsDisabled(t *testing.T) {
	testCases := []struct {
		name  string
		value string
	}{
		{name: "numeric opt-out", value: "1"},
		{name: "textual opt-out", value: trueString},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(doNotTrackEnv, tc.value)
			out := stubFlushCommand(t, `cat > "$1"`)

			reportCommand(newReportableCommand(), time.Second, nil)

			assertNoChildOutput(t, out, "no telemetry child may be spawned when opted out")
		})
	}
}

// TestReportCommandSkipsUnresolvedCommands covers `chainloop` on its own and unknown
// subcommands, where cobra hands back the root command and there is no command path to
// report.
func TestReportCommandSkipsUnresolvedCommands(t *testing.T) {
	t.Setenv(doNotTrackEnv, "")
	out := stubFlushCommand(t, `cat > "$1"`)

	reportCommand(newTestRootCommand(), time.Second, nil)

	assertNoChildOutput(t, out, "the root command has no command path and must not be reported")
}
