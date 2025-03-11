//
// Copyright 2025 The Chainloop Authors.
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

// columnNotFoundSQLState is the PostgresSQL state for column not found error.
// Reference: https://www.postgresql.org/docs/8.2/errcodes-appendix.html
const columnNotFoundSQLState = "42703"

// columnNotExistRegex extracts column name from the error message
var columnNotExistRegex = regexp.MustCompile(`column "([^"]+)" does not exist`)

// ColumnNotFoundError represents an error related to a column not being found
type ColumnNotFoundError struct {
	err    error
	column string
}

func (e ColumnNotFoundError) Error() string {
	// extract the column from the error
	matches := columnNotExistRegex.FindStringSubmatch(e.err.Error())
	if len(matches) == 2 {
		e.column = matches[1]
	}

	return fmt.Sprintf("json filter column '%s' does not exist", e.column)
}

// Unwrap returns the wrapped error
func (e *ColumnNotFoundError) Unwrap() error {
	return e.err
}

// NewColumnNotFoundError creates a new ColumnNotFoundError.
func NewColumnNotFoundError(err error) ColumnNotFoundError {
	return ColumnNotFoundError{err: err}
}

// IsColumnNotFoundError checks if the error is a ColumnNotFoundError.
func IsColumnNotFoundError(err error) bool {
	return errors.As(err, &ColumnNotFoundError{})
}

// IsJSONFilterError checks if the error is related to a JSON filter.
func IsJSONFilterError(err error, filters []*JSONFilter) bool {
	if err == nil {
		return false
	}

	// Check if the error is related to a column not being found.
	matches := columnNotExistRegex.FindStringSubmatch(err.Error())
	if !strings.Contains(err.Error(), columnNotFoundSQLState) || len(matches) != 2 {
		return false
	}

	// Check if the column is part of the JSON filters.
	for _, filter := range filters {
		if matches[1] == filter.Column {
			return true
		}
	}

	return false
}

// JSONOperator represents supported JSON filter operators.
type JSONOperator string

const (
	OpEQ        JSONOperator = "eq"
	OpNEQ       JSONOperator = "neq"
	OpGT        JSONOperator = "gt"
	OpGTE       JSONOperator = "gte"
	OpLT        JSONOperator = "lt"
	OpLTE       JSONOperator = "lte"
	OpHasKey    JSONOperator = "haskey"
	OpIsNull    JSONOperator = "isnull"
	OpIsNotNull JSONOperator = "isnotnull"
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

	// Convert the dot notation to the path that Ent expects.
	dotPath := sqljson.DotPath(jsonFilter.FieldPath)

	// Build the predicate based on the operator.
	switch jsonFilter.Operator {
	case OpEQ:
		return sqljson.ValueEQ(jsonFilter.Column, jsonFilter.Value, dotPath), nil
	case OpNEQ:
		return sqljson.ValueNEQ(jsonFilter.Column, jsonFilter.Value, dotPath), nil
	case OpGT:
		return sqljson.ValueGT(jsonFilter.Column, jsonFilter.Value, dotPath), nil
	case OpGTE:
		return sqljson.ValueGTE(jsonFilter.Column, jsonFilter.Value, dotPath), nil
	case OpLT:
		return sqljson.ValueLT(jsonFilter.Column, jsonFilter.Value, dotPath), nil
	case OpLTE:
		return sqljson.ValueLTE(jsonFilter.Column, jsonFilter.Value, dotPath), nil
	case OpHasKey:
		return sqljson.HasKey(jsonFilter.Column, dotPath), nil
	case OpIsNull:
		return sqljson.ValueIsNull(jsonFilter.Column, dotPath), nil
	case OpIsNotNull:
		return sqljson.ValueIsNotNull(jsonFilter.Column, dotPath), nil
	default:
		return nil, fmt.Errorf("unsupported operator: %s", jsonFilter.Operator)
	}
}
