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

// Package sigcheck parses Sysinternals sigcheck CSV/TSV output into a
// JSON-friendly structure for policy evaluation.
// https://learn.microsoft.com/en-us/sysinternals/downloads/sigcheck
package sigcheck

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Report is a parsed sigcheck report: the CSV header columns and one map per
// data row, keyed by header column.
type Report struct {
	Header []string
	// Rows holds one map per data row, keyed by header column. If the header
	// contains duplicate column names, the last column wins.
	Rows []map[string]string
}

// Parse decodes sigcheck CSV/TSV output. It strips/decodes UTF-8 and UTF-16
// byte-order marks and auto-detects whether the delimiter is a comma or a tab.
func Parse(raw []byte) (*Report, error) {
	data, err := decode(raw)
	if err != nil {
		return nil, fmt.Errorf("decoding sigcheck output: %w", err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("empty sigcheck report")
	}

	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = detectDelimiter(data)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1 // tolerate ragged rows

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing sigcheck CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, errors.New("sigcheck report has no header row")
	}

	header := records[0]
	report := &Report{Header: header, Rows: make([]map[string]string, 0, len(records)-1)}
	for _, rec := range records[1:] {
		row := make(map[string]string, len(header))
		for i, col := range header {
			if i < len(rec) {
				row[col] = rec[i]
			} else {
				row[col] = ""
			}
		}
		report.Rows = append(report.Rows, row)
	}

	return report, nil
}

// JSON marshals the report rows as a JSON array. A header-only report marshals
// to "[]".
func (r *Report) JSON() ([]byte, error) {
	if r.Rows == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(r.Rows)
}

// HasColumns reports whether every named column is present in the header.
func (r *Report) HasColumns(cols ...string) bool {
	set := make(map[string]struct{}, len(r.Header))
	for _, h := range r.Header {
		set[strings.TrimSpace(h)] = struct{}{}
	}
	for _, c := range cols {
		if _, ok := set[strings.TrimSpace(c)]; !ok {
			return false
		}
	}
	return true
}

// decode normalizes the input to UTF-8, using the BOM (if any) to detect
// UTF-16; defaults to UTF-8 when no BOM is present, stripping a UTF-8 BOM.
func decode(raw []byte) ([]byte, error) {
	dec := unicode.BOMOverride(unicode.UTF8.NewDecoder())
	out, _, err := transform.Bytes(dec, raw)
	return out, err
}

// detectDelimiter inspects the header line and picks the delimiter (comma or
// tab) that appears more often outside of quoted fields. sigcheck's comma
// output quotes every field, so a tab inside a quoted path or description must
// not be mistaken for a TSV separator. Defaults to comma.
func detectDelimiter(data []byte) rune {
	line := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		line = data[:i]
	}

	var commas, tabs int
	inQuotes := false
	for _, b := range line {
		switch b {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				commas++
			}
		case '\t':
			if !inQuotes {
				tabs++
			}
		}
	}

	if tabs > commas {
		return '\t'
	}
	return ','
}
