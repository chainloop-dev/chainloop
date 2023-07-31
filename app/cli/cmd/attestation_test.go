//
// Copyright 2023 The Chainloop Authors.
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

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractAnnotations(t *testing.T) {
	testCases := []struct {
		input   []string
		want    map[string]string
		wantErr bool
	}{
		{
			input: []string{
				"foo=bar",
				"baz=qux",
			},
			want: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
			wantErr: false,
		},
		{
			input: []string{
				"foo=bar",
				"baz",
			},
			wantErr: true,
		},
		{
			input: []string{
				"foo=bar",
				"baz=qux",
				"foo=bar",
			},
			want: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
			wantErr: false,
		},
		{
			input: []string{
				"foo=bar",
				"baz=qux=qux",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		got, err := extractAnnotations(tc.input)
		if tc.wantErr {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)
		assert.Equal(t, tc.want, got)
	}
}
