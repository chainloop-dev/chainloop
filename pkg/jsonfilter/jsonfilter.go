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
	"strings"

	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqljson"
)

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
