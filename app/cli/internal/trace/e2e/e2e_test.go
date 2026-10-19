//go:build e2e

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

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Environment of a run of the suite.
const (
	// envHarness, envModel and envFeature filter a manual run. Each is a
	// comma-separated list. A filtered run does not become the previous result.
	envHarness = "CHAINLOOP_E2E_HARNESS"
	envModel   = "CHAINLOOP_E2E_MODEL"
	envFeature = "CHAINLOOP_E2E_FEATURE"
	// envResultsDir is where the results are stored. The default is the
	// results folder of this package, which git ignores.
	envResultsDir = "CHAINLOOP_E2E_RESULTS_DIR"
	// envWorkDir keeps the workspaces and the harness output after the run.
	// The default is a temporary directory that the test deletes.
	envWorkDir = "CHAINLOOP_E2E_WORK_DIR"
)

// TestTraceMatrix runs the end-to-end suite of Chainloop Trace with real
// harnesses and models (spec issue-3635). It builds the chainloop CLI, runs the
// scenarios for each harness and model of config.yaml, stores the result
// matrix, and compares it with the previous full run.
//
// It fails when a cell that the feature table marks "Yes" or "Partial" does
// not pass, or when a scenario could not run. A feature row with no check is
// logged, not failed.
func TestTraceMatrix(t *testing.T) {
	ctx := t.Context()

	f, err := os.Open(traceREADMEPath)
	require.NoError(t, err)
	exp, err := ParseExpected(f)
	f.Close()
	require.NoError(t, err)

	cfg, err := LoadConfig(nil)
	require.NoError(t, err)

	bin, version := buildChainloop(t)

	workDir := os.Getenv(envWorkDir)
	if workDir == "" {
		workDir = t.TempDir()
	}

	runner := &Runner{
		Expected:     exp,
		Config:       cfg,
		Harnesses:    []Harness{&Claude{}},
		Filter:       Filter{Harnesses: envList(envHarness), Models: envList(envModel), Features: envList(envFeature)},
		ChainloopBin: bin,
		CLIVersion:   version,
		WorkDir:      workDir,
		Logf:         t.Logf,
	}

	res, err := runner.Run(ctx)
	require.NoError(t, err)

	resultsDir := os.Getenv(envResultsDir)
	if resultsDir == "" {
		resultsDir = "results"
	}
	store := ResultStore{Dir: resultsDir}
	prev, err := store.Previous()
	require.NoError(t, err)
	changes := Compare(prev, res)
	path, err := store.Save(res)
	require.NoError(t, err)

	t.Logf("result matrix, stored in %s:\n\n%s", path, Markdown(res))
	for _, c := range changes.Regressions {
		t.Logf("REGRESSION: %s / %s %s / %s: %s -> %s (%d/%d): %s", c.Cell.Feature, c.Cell.Harness, c.Cell.HarnessVersion, c.Cell.Model, c.Previous, c.Cell.Status, c.Cell.Passes, c.Cell.Runs, c.Cell.Reason)
	}
	for _, c := range changes.Improvements {
		t.Logf("IMPROVEMENT: %s / %s %s / %s passes, the table says Unknown", c.Cell.Feature, c.Cell.Harness, c.Cell.HarnessVersion, c.Cell.Model)
	}
	// A row with no check is a gap of the suite, which a new check closes. A
	// scenario that could not run makes the result unreliable, so it fails.
	for _, c := range res.Problems() {
		if c.Status == StatusNoCheck {
			t.Logf("suite gap: %s / %s / %s: %s", c.Feature, c.Harness, c.Model, c.Reason)
			continue
		}
		assert.Failf(t, "suite problem", "%s / %s / %s: %s: %s", c.Feature, c.Harness, c.Model, c.Status, c.Reason)
	}
	for _, c := range res.Cells {
		if c.Status == StatusFail && (c.Expected == SupportYes || c.Expected == SupportPartial) {
			assert.Failf(t, "regression of a supported feature", "%s / %s %s / %s: marked %s but fails (%d/%d): %s", c.Feature, c.Harness, c.HarnessVersion, c.Model, c.Expected, c.Passes, c.Runs, c.Reason)
		}
	}
}

// buildChainloop builds the chainloop CLI under test and returns its path and
// its version.
func buildChainloop(t *testing.T) (string, string) {
	t.Helper()

	version := "e2e"
	if out, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output(); err == nil {
		version = strings.TrimSpace(string(out))
	}

	bin := filepath.Join(t.TempDir(), "chainloop")
	cmd := exec.Command("go", "build",
		"-ldflags", "-X github.com/chainloop-dev/chainloop/app/cli/cmd.Version="+version,
		"-o", bin, "github.com/chainloop-dev/chainloop/app/cli")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build chainloop: %s", out)

	return bin, version
}

func envList(name string) []string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
