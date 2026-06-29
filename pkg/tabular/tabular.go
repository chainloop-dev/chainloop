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

// Package tabular parses delimited tabular text (CSV or TSV) into a
// header-plus-rows structure. It decodes UTF-8/UTF-16 byte-order marks and
// auto-detects whether the delimiter is a comma or a tab, so it handles output
// from tools like Sysinternals sigcheck as well as generic CSV/TSV files.
package tabular

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

// Table is parsed tabular data: the header columns and one map per data row,
// keyed by header column.
type Table struct {
	Header []string
	// Rows holds one map per data row, keyed by header column. If the header
	// contains duplicate column names, the last column wins.
	Rows []map[string]string
}

// Parse decodes delimited CSV/TSV text. It strips/decodes UTF-8 and UTF-16
// byte-order marks and auto-detects whether the delimiter is a comma or a tab.
func Parse(raw []byte) (*Table, error) {
	data, err := decode(raw)
	if err != nil {
		return nil, fmt.Errorf("decoding tabular input: %w", err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("empty tabular input")
	}

	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = detectDelimiter(data)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1 // tolerate ragged rows

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV/TSV: %w", err)
	}
	if len(records) == 0 {
		return nil, errors.New("tabular input has no header row")
	}

	header := records[0]
	table := &Table{Header: header, Rows: make([]map[string]string, 0, len(records)-1)}
	for _, rec := range records[1:] {
		row := make(map[string]string, len(header))
		for i, col := range header {
			if i < len(rec) {
				row[col] = rec[i]
			} else {
				row[col] = ""
			}
		}
		table.Rows = append(table.Rows, row)
	}

	return table, nil
}

// JSON marshals the table rows as a JSON array. A header-only table marshals
// to "[]".
func (t *Table) JSON() ([]byte, error) {
	if t.Rows == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(t.Rows)
}

// HasColumns reports whether every named column is present in the header.
func (t *Table) HasColumns(cols ...string) bool {
	set := make(map[string]struct{}, len(t.Header))
	for _, h := range t.Header {
		set[strings.TrimSpace(h)] = struct{}{}
	}
	for _, c := range cols {
		if _, ok := set[strings.TrimSpace(c)]; !ok {
			return false
		}
	}
	return true
}

// Column returns the values of the column whose header matches name
// case-insensitively (trimming surrounding whitespace), with empty and
// whitespace-only cells dropped, and whether such a column exists.
func (t *Table) Column(name string) ([]string, bool) {
	var header string
	found := false
	for _, h := range t.Header {
		if strings.EqualFold(strings.TrimSpace(h), strings.TrimSpace(name)) {
			header = h
			found = true
			break
		}
	}
	if !found {
		return nil, false
	}

	values := make([]string, 0, len(t.Rows))
	for _, row := range t.Rows {
		if v := strings.TrimSpace(row[header]); v != "" {
			values = append(values, v)
		}
	}
	return values, true
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
	line, _, _ := bytes.Cut(data, []byte{'\n'})

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
