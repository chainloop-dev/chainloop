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

// Package e2e is the end-to-end test suite of Chainloop Trace (spec
// issue-3635). It runs real coding agents in headless mode, in throwaway
// repositories with Trace in local storage mode, and builds a result matrix of
// harness, model and feature. The expected matrix is the feature table of the
// Trace README.
package e2e

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// traceREADMEPath is the Trace README, relative to this package. It holds the
// feature table that is the expected matrix (D-002).
const traceREADMEPath = "../README.md"

// Support is the value of one cell of the feature table.
type Support string

const (
	SupportYes           Support = "Yes"
	SupportPartial       Support = "Partial"
	SupportNo            Support = "No"
	SupportUnknown       Support = "Unknown"
	SupportNotApplicable Support = "Not applicable"
)

// Tested reports whether the suite tests a cell with this value. "No" and "Not
// applicable" cells are skipped (R-002).
func (s Support) Tested() bool {
	switch s {
	case SupportYes, SupportPartial, SupportUnknown:
		return true
	default:
		return false
	}
}

func parseSupport(v string) (Support, bool) {
	s := Support(v)
	switch s {
	case SupportYes, SupportPartial, SupportNo, SupportUnknown, SupportNotApplicable:
		return s, true
	default:
		return "", false
	}
}

// Expected is the expected matrix read from the feature table.
type Expected struct {
	// Features are the feature names in the order of the README.
	Features []string
	// Harnesses are the harness column names in the order of the README.
	Harnesses []string
	cells     map[string]map[string]Support
}

// Support returns the value of the cell of a feature and a harness column. The
// second result is false when the table has no such cell.
func (e *Expected) Support(feature, harness string) (Support, bool) {
	s, ok := e.cells[feature][harness]
	return s, ok
}

// featureHeader is the start of the header row of a feature table. Other
// tables of the README, for example how each agent is hooked in, do not start
// with it.
var featureHeader = []string{"Feature", "What it means"}

// ParseExpected reads the feature tables of the Trace README. All feature
// tables must have the same harness columns.
func ParseExpected(r io.Reader) (*Expected, error) {
	exp := &Expected{cells: make(map[string]map[string]Support)}

	var (
		columns []string // harness columns of the current table, nil outside one
		inTable bool
	)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") {
			inTable, columns = false, nil
			continue
		}

		cells := splitRow(line)
		if !inTable {
			inTable = true
			if len(cells) > len(featureHeader) && cells[0] == featureHeader[0] && cells[1] == featureHeader[1] {
				columns = cells[len(featureHeader):]
				if err := exp.addHarnesses(columns); err != nil {
					return nil, err
				}
			}
			continue
		}

		if columns == nil || isSeparatorRow(cells) {
			continue
		}

		if err := exp.addRow(cells, columns); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read feature table: %w", err)
	}

	if len(exp.Features) == 0 {
		return nil, errors.New("no feature table found")
	}

	return exp, nil
}

func (e *Expected) addHarnesses(columns []string) error {
	if e.Harnesses == nil {
		e.Harnesses = columns
		return nil
	}
	if strings.Join(e.Harnesses, "|") != strings.Join(columns, "|") {
		return fmt.Errorf("feature tables have different harness columns: %v and %v", e.Harnesses, columns)
	}
	return nil
}

func (e *Expected) addRow(cells, columns []string) error {
	feature := cells[0]
	if want := len(featureHeader) + len(columns); len(cells) != want {
		return fmt.Errorf("feature %q has %d cells, want %d", feature, len(cells), want)
	}
	if _, dup := e.cells[feature]; dup {
		return fmt.Errorf("feature %q is listed twice", feature)
	}

	row := make(map[string]Support, len(columns))
	for i, harness := range columns {
		raw := cells[len(featureHeader)+i]
		s, ok := parseSupport(raw)
		if !ok {
			return fmt.Errorf("feature %q, column %q: unknown value %q", feature, harness, raw)
		}
		row[harness] = s
	}

	e.cells[feature] = row
	e.Features = append(e.Features, feature)
	return nil
}

// splitRow splits a markdown table row into its trimmed cells, with bold
// markers removed.
func splitRow(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	parts := strings.Split(line, "|")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(strings.ReplaceAll(p, "**", ""))
	}
	return parts
}

func isSeparatorRow(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}
