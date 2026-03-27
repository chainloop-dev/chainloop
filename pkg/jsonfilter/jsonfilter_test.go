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

package jsonfilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEntSelectorFromJSONFilter(t *testing.T) {
	tests := []struct {
		name    string
		filter  *JSONFilter
		wantErr string
	}{
		{
			name:    "missing column",
			filter:  &JSONFilter{Operator: OpEQ, Value: "foo"},
			wantErr: "invalid filter: column and operator are required",
		},
		{
			name:    "missing operator",
			filter:  &JSONFilter{Column: "metadata", Value: "foo"},
			wantErr: "invalid filter: column and operator are required",
		},
		{
			name:    "unsupported operator",
			filter:  &JSONFilter{Column: "metadata", Operator: "gt", Value: "foo"},
			wantErr: "unsupported operator: gt",
		},
		{
			name:   "eq operator with string value",
			filter: &JSONFilter{Column: "metadata", FieldPath: "name", Operator: OpEQ, Value: "foo"},
		},
		{
			name:   "eq operator with nested field path",
			filter: &JSONFilter{Column: "metadata", FieldPath: "labels.env", Operator: OpEQ, Value: "prod"},
		},
		{
			name:   "eq operator with empty field path",
			filter: &JSONFilter{Column: "metadata", FieldPath: "", Operator: OpEQ, Value: "foo"},
		},
		{
			name:   "neq operator with string value",
			filter: &JSONFilter{Column: "metadata", FieldPath: "name", Operator: OpNEQ, Value: "bar"},
		},
		{
			name:   "in operator with single value",
			filter: &JSONFilter{Column: "metadata", FieldPath: "env", Operator: OpIN, Value: "prod"},
		},
		{
			name:   "in operator with comma-separated values",
			filter: &JSONFilter{Column: "metadata", FieldPath: "env", Operator: OpIN, Value: "prod,staging,dev"},
		},
		{
			name:   "in operator trims spaces around values",
			filter: &JSONFilter{Column: "metadata", FieldPath: "env", Operator: OpIN, Value: "prod, staging , dev"},
		},
		{
			name:    "in operator with non-string value",
			filter:  &JSONFilter{Column: "metadata", FieldPath: "env", Operator: OpIN, Value: 42},
			wantErr: "invalid value for 'in' operator: must be a slice of strings",
		},
		{
			name:    "in operator with slice value instead of string",
			filter:  &JSONFilter{Column: "metadata", FieldPath: "env", Operator: OpIN, Value: []string{"prod", "dev"}},
			wantErr: "invalid value for 'in' operator: must be a slice of strings",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pred, err := BuildEntSelectorFromJSONFilter(tc.filter)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				assert.Nil(t, pred)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, pred)
			}
		})
	}
}
