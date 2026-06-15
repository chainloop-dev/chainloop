//
// Copyright 2025-2026 The Chainloop Authors.
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

package jsonfilter

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqljson"
)

// fieldPathRegexp matches a safe JSON field path expressed in dot notation.
// A path is a dot-separated sequence of identifiers (starting with a letter or
// underscore) optionally followed by numeric array indices, e.g. "name",
// "labels.env" or "items[0].name".
//
// This allowlist is security-critical: the underlying ent sqljson helpers
// concatenate path segments verbatim into single-quoted PostgreSQL JSON path
// literals without escaping, so any character outside this set (e.g. a single
// quote) would allow breaking out of the literal and injecting arbitrary SQL.
var fieldPathRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*|\[[0-9]+\])*$`)

// columnRegexp matches a safe SQL column identifier (a single unqualified
// identifier such as "metadata").
//
// This allowlist is security-critical: the underlying ent sqljson helpers emit
// the column verbatim into the query and, for PostgreSQL, a column containing a
// double quote bypasses ent's identifier quoting entirely and is written raw,
// allowing SQL injection identical to the field-path case. Restricting the
// column to a bare identifier removes every character an injection would need.
var columnRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// JSONOperator represents supported JSON filter operators.
type JSONOperator string

const (
	OpEQ  JSONOperator = "eq"
	OpNEQ JSONOperator = "neq"
	OpIN  JSONOperator = "in"
)

// JSONFilter represents a filter for JSON fields.
type JSONFilter struct {
	// Column is the name of the JSON column.
	Column string
	// FieldPath is the path to the field in the JSON object.
	// It's the inner path within the json column. It accepts dot notation.
	// Example:
	// "metadata.name" for {"metadata": {"name": "foo"}}
	FieldPath string
	// Value is the value to compare against.
	Value any
	// Operator is the comparison operator.
	Operator JSONOperator
}

// BuildEntSelectorFromJSONFilter constructs an Ent SQL predicate for JSON filtering.
func BuildEntSelectorFromJSONFilter(jsonFilter *JSONFilter) (*entsql.Predicate, error) {
	if jsonFilter.Column == "" || jsonFilter.Operator == "" {
		return nil, errors.New("invalid filter: column and operator are required")
	}

	// Validate the column before it reaches the SQL builder. Like the field
	// path, the column is emitted into the query by the ent sqljson helpers
	// without reliable escaping, so an unsafe value would allow SQL injection.
	if err := validateColumn(jsonFilter.Column); err != nil {
		return nil, err
	}

	// Validate the field path before it reaches the SQL builder. The ent
	// sqljson helpers concatenate the path segments unescaped into the query,
	// so an unsafe value would allow SQL injection.
	if err := validateFieldPath(jsonFilter.FieldPath); err != nil {
		return nil, err
	}

	// Convert the dot notation to the path that Ent expects.
	dotPath := sqljson.DotPath(jsonFilter.FieldPath)

	// Build the predicate based on the operator.
	switch jsonFilter.Operator {
	case OpEQ:
		return sqljson.ValueEQ(jsonFilter.Column, jsonFilter.Value, dotPath), nil
	case OpNEQ:
		return sqljson.ValueNEQ(jsonFilter.Column, jsonFilter.Value, dotPath), nil
	case OpIN:
		// Parse the jsonFilter.Value into a string
		castedValue, ok := jsonFilter.Value.(string)
		if !ok {
			return nil, errors.New("invalid value for 'in' operator: must be a slice of strings")
		}

		// Split the string into a slice of strings by comma.
		values := strings.Split(castedValue, ",")

		// Trim the spaces from each value and transform them into a slice of any needed for the predicate.
		inputValue := make([]any, 0, len(values))
		for _, value := range values {
			inputValue = append(inputValue, strings.TrimSpace(value))
		}

		// Create the predicate.
		return sqljson.ValueIn(jsonFilter.Column, inputValue, dotPath), nil
	default:
		return nil, fmt.Errorf("unsupported operator: %s", jsonFilter.Operator)
	}
}

// validateColumn ensures the JSON column is a bare SQL identifier before it is
// emitted into the query by the ent sqljson helpers. Callers are expected to set
// it to a known column constant, but validating here keeps safety from depending
// on every caller remembering to do so.
func validateColumn(column string) error {
	if !columnRegexp.MatchString(column) {
		return fmt.Errorf("invalid column %q: must be a valid identifier", column)
	}

	return nil
}

// validateFieldPath ensures the JSON field path only contains safe characters
// before it is concatenated into the SQL query by the ent sqljson helpers.
// An empty path is allowed: it targets the column itself and is not injectable.
func validateFieldPath(fieldPath string) error {
	if fieldPath == "" {
		return nil
	}

	if !fieldPathRegexp.MatchString(fieldPath) {
		return fmt.Errorf("invalid field path %q: must be dot-separated identifiers with optional numeric array indices", fieldPath)
	}

	return nil
}
