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
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluate(t *testing.T) {
	pass := func(*Session) error { return nil }
	fail := func(*Session) error { return errors.New("boom") }
	notCovered := func(*Session) error { return fmt.Errorf("%w: headless", errNotCovered) }
	stored := &Session{Stored: &StoredSession{}}

	// failOn fails the given runs of a scenario, counted from 1.
	failOn := func(runs ...int) func(*Session) error {
		return func(s *Session) error {
			for _, r := range runs {
				if s.Model == fmt.Sprint(r) {
					return errors.New("boom")
				}
			}
			return nil
		}
	}
	numbered := func(n int) []sessionResult {
		out := make([]sessionResult, n)
		for i := range out {
			out[i] = sessionResult{session: &Session{Model: fmt.Sprint(i + 1), Stored: &StoredSession{}}}
		}
		return out
	}

	testCases := []struct {
		name       string
		checks     []Check
		repeat     Repeat
		sessions   map[string][]sessionResult
		wantStatus Status
		wantPasses int
	}{
		{
			name:       "single run passes",
			checks:     []Check{{"a", pass}},
			repeat:     Repeat{1, 1},
			sessions:   map[string][]sessionResult{"a": {{session: stored}}},
			wantStatus: StatusPass,
			wantPasses: 1,
		},
		{
			name:       "all checks must pass",
			checks:     []Check{{"a", pass}, {"a", fail}},
			repeat:     Repeat{1, 1},
			sessions:   map[string][]sessionResult{"a": {{session: stored}}},
			wantStatus: StatusFail,
		},
		{
			name:       "two of three runs pass",
			checks:     []Check{{"a", failOn(2)}},
			repeat:     Repeat{3, 2},
			sessions:   map[string][]sessionResult{"a": numbered(3)},
			wantStatus: StatusPass,
			wantPasses: 2,
		},
		{
			name:       "one of three runs passes",
			checks:     []Check{{"a", failOn(1, 3)}},
			repeat:     Repeat{3, 2},
			sessions:   map[string][]sessionResult{"a": numbered(3)},
			wantStatus: StatusFail,
			wantPasses: 1,
		},
		{
			name:       "scenario that could not run is a suite error",
			checks:     []Check{{"a", pass}},
			repeat:     Repeat{1, 1},
			sessions:   map[string][]sessionResult{"a": {{err: errors.New("no harness")}}},
			wantStatus: StatusError,
		},
		{
			name:       "check that cannot see the feature",
			checks:     []Check{{"a", notCovered}},
			repeat:     Repeat{1, 1},
			sessions:   map[string][]sessionResult{"a": {{session: stored}}},
			wantStatus: StatusNotCovered,
		},
		{
			name:       "no stored session fails",
			checks:     []Check{{"a", pass}},
			repeat:     Repeat{1, 1},
			sessions:   map[string][]sessionResult{"a": {{session: &Session{}}}},
			wantStatus: StatusFail,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluate(plannedCell{feature: Feature{Checks: tc.checks}, repeat: tc.repeat}, tc.sessions)
			assert.Equal(t, tc.wantStatus, got.Status, got.Reason)
			assert.Equal(t, tc.wantPasses, got.Passes)
			assert.Equal(t, tc.repeat.Runs, got.Runs)
		})
	}
}

// fakeHarness fails to report its version, so the runner tests the planning
// of the cells and runs no session.
type fakeHarness struct{}

func (fakeHarness) Column() string    { return HarnessClaudeCode }
func (fakeHarness) TraceFlag() string { return "--claude" }
func (fakeHarness) Version(context.Context) (string, error) {
	return "", errors.New("not installed")
}
func (fakeHarness) Run(context.Context, RunRequest) (*RunOutput, error) {
	return nil, errors.New("not installed")
}
func (fakeHarness) HookConfig() HookConfig           { return HookConfig{} }
func (fakeHarness) DisableSessionStart(string) error { return errors.New("not installed") }

func TestRunnerPlansCells(t *testing.T) {
	exp, err := ParseExpected(strings.NewReader(`| Feature | What it means | Claude Code | Cursor |
|---|---|---|---|
| Transcript | x | Yes | Yes |
| Subagents | x | No | No |
| Attestation at push | x | Yes | Yes |
| Brand new feature | x | Unknown | Unknown |
`))
	require.NoError(t, err)
	cfg, err := LoadConfig(nil)
	require.NoError(t, err)

	r := &Runner{Expected: exp, Config: cfg, Harnesses: []Harness{fakeHarness{}}, WorkDir: t.TempDir(), Logf: t.Logf}
	res, err := r.Run(context.Background())
	require.NoError(t, err)

	got := make(map[string]Status)
	for _, c := range res.Cells {
		got[c.Feature] = c.Status
	}
	assert.Equal(t, map[string]Status{
		featTranscript:        StatusError, // the harness is not installed
		"Subagents":           StatusSkipped,
		"Attestation at push": StatusNotCovered,
		"Brand new feature":   StatusNoCheck,
	}, got)
	assert.True(t, res.Full)
}

// TestFeaturesMatchREADME keeps the registered checks in line with the feature
// table: a check whose feature was renamed in the table would never run.
func TestFeaturesMatchREADME(t *testing.T) {
	f, err := os.Open(traceREADMEPath)
	require.NoError(t, err)
	defer f.Close()
	exp, err := ParseExpected(f)
	require.NoError(t, err)

	for name, def := range features {
		assert.Contains(t, exp.Features, name, "registered feature is not in the table")
		for _, chk := range def.Checks {
			assert.Contains(t, scenarios, chk.Scenario, "feature %q", name)
		}
	}
}
