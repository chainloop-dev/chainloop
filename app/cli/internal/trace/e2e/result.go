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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Status is the outcome of one cell of the result matrix.
type Status string

const (
	StatusPass Status = "pass"
	StatusFail Status = "fail"
	// StatusSkipped is a cell that the feature table marks "No" or "Not
	// applicable" (R-002).
	StatusSkipped Status = "skipped"
	// StatusNotCovered is a cell that the suite cannot test: the feature needs
	// a control plane (D-007), or the headless mode of the harness cannot show
	// it (R-004).
	StatusNotCovered Status = "not_covered"
	// StatusNoCheck is a feature row with no registered check. It is a suite
	// problem (R-002, R-008).
	StatusNoCheck Status = "no_check"
	// StatusError is a cell whose scenario could not run, for example because
	// the harness did not start. It is a suite problem (R-008).
	StatusError Status = "error"
)

// resultSchemaVersion is the version of the stored result format.
const resultSchemaVersion = 1

// Cell is one cell of the result matrix: a feature for a harness and a model.
type Cell struct {
	Feature        string  `json:"feature"`
	Harness        string  `json:"harness"`
	HarnessVersion string  `json:"harness_version,omitempty"`
	Model          string  `json:"model"`
	Expected       Support `json:"expected"`
	Status         Status  `json:"status"`
	// Passes and Runs give the pass rate of the cell (R-006).
	Passes int `json:"passes"`
	Runs   int `json:"runs"`
	// Reason tells why the cell did not pass, or why it was not tested.
	Reason string `json:"reason,omitempty"`
}

type cellKey struct {
	feature, harness, model string
}

func (c Cell) key() cellKey {
	return cellKey{c.Feature, c.Harness, c.Model}
}

// Result is the result matrix of one run.
type Result struct {
	SchemaVersion int       `json:"schema_version"`
	StartedAt     time.Time `json:"started_at"`
	CLIVersion    string    `json:"cli_version"`
	// Full is true for a run with no filter. Only a full run becomes the
	// previous result of the next comparison (R-009).
	Full  bool   `json:"full"`
	Cells []Cell `json:"cells"`
}

// Problems returns the cells that show a problem of the suite, not of Trace: a
// feature with no check, or a scenario that could not run (R-008).
func (r *Result) Problems() []Cell {
	var out []Cell
	for _, c := range r.Cells {
		if c.Status == StatusNoCheck || c.Status == StatusError {
			out = append(out, c)
		}
	}
	return out
}

// Change is a cell whose state changed from the previous run.
type Change struct {
	Cell Cell
	// Previous is the status in the previous run, empty when the cell is new.
	Previous Status
}

// Changes are the cells to report after a run (R-007).
type Changes struct {
	// Regressions are cells that passed in the previous run and fail now.
	Regressions []Change
	// Improvements are cells marked "Unknown" that pass now and did not pass
	// in the previous run, so the team can update the table.
	Improvements []Change
}

// Empty reports whether there is nothing to report.
func (c Changes) Empty() bool {
	return len(c.Regressions) == 0 && len(c.Improvements) == 0
}

// Compare compares the current result with the previous one. prev is nil when
// there is no previous result. A cell that failed in the previous run too is not
// reported again (R-007, D-005).
func Compare(prev, cur *Result) Changes {
	before := make(map[cellKey]Status)
	if prev != nil {
		for _, c := range prev.Cells {
			before[c.key()] = c.Status
		}
	}

	var changes Changes
	for _, c := range cur.Cells {
		was := before[c.key()]
		switch {
		case was == StatusPass && c.Status == StatusFail:
			changes.Regressions = append(changes.Regressions, Change{Cell: c, Previous: was})
		case c.Expected == SupportUnknown && c.Status == StatusPass && was != StatusPass:
			changes.Improvements = append(changes.Improvements, Change{Cell: c, Previous: was})
		}
	}

	return changes
}

const (
	// latestResultFile is the result of the last full run: the previous result
	// of the next comparison.
	latestResultFile = "latest.json"
	// runsDir keeps the result of every run, named by its start time.
	runsDir = "runs"
)

// ResultStore keeps the result matrices of the runs in a directory.
type ResultStore struct {
	Dir string
}

// Previous returns the result of the last full run, or nil when there is none.
func (s ResultStore) Previous() (*Result, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, latestResultFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read previous result: %w", err)
	}

	var res Result
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("decode previous result: %w", err)
	}
	return &res, nil
}

// Save writes the result to the history and, for a full run, makes it the
// previous result of the next run. It returns the path of the history file.
func (s ResultStore) Save(res *Result) (string, error) {
	res.SchemaVersion = resultSchemaVersion
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode result: %w", err)
	}

	dir := filepath.Join(s.Dir, runsDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create results directory: %w", err)
	}

	path := filepath.Join(dir, res.StartedAt.UTC().Format("20060102T150405Z")+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write result: %w", err)
	}

	if res.Full {
		if err := os.WriteFile(filepath.Join(s.Dir, latestResultFile), data, 0o600); err != nil {
			return "", fmt.Errorf("write previous result: %w", err)
		}
	}

	return path, nil
}

// Markdown renders the result matrix as a markdown table: one row per feature
// and one column per harness and model (R-010).
func Markdown(res *Result) string {
	type column struct{ harness, model string }

	var (
		features []string
		columns  []column
		labels   = make(map[column]string)
		cells    = make(map[cellKey]Cell)
		seenFeat = make(map[string]bool)
	)
	for _, c := range res.Cells {
		if !seenFeat[c.Feature] {
			seenFeat[c.Feature] = true
			features = append(features, c.Feature)
		}
		col := column{c.Harness, c.Model}
		if _, ok := labels[col]; !ok {
			columns = append(columns, col)
			labels[col] = strings.TrimSpace(c.Harness+" "+c.HarnessVersion) + " / " + c.Model
		}
		cells[c.key()] = c
	}

	var b strings.Builder
	b.WriteString("| Feature |")
	for _, col := range columns {
		b.WriteString(" " + labels[col] + " |")
	}
	b.WriteString("\n|---|")
	for range columns {
		b.WriteString("---|")
	}
	b.WriteString("\n")

	for _, f := range features {
		b.WriteString("| " + f + " |")
		for _, col := range columns {
			c, ok := cells[cellKey{f, col.harness, col.model}]
			if !ok {
				b.WriteString("  |")
				continue
			}
			b.WriteString(" " + cellLabel(c) + " |")
		}
		b.WriteString("\n")
	}

	return b.String()
}

// cellLabel shows the status of a cell, with the pass rate when the cell ran
// more than once.
func cellLabel(c Cell) string {
	label := strings.ReplaceAll(string(c.Status), "_", " ")
	if c.Runs > 1 && (c.Status == StatusPass || c.Status == StatusFail) {
		label += fmt.Sprintf(" %d/%d", c.Passes, c.Runs)
	}
	return label
}
