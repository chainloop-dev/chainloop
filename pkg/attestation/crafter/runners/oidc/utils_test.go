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

package oidc

import (
	"testing"
)

func Test_compareStringSlice(t *testing.T) {
	tests := []struct {
		name string
		s1   []string
		s2   []string
		want bool
	}{
		{
			name: "empty slices",
			s1:   []string{},
			s2:   []string{},
			want: true,
		},
		{
			name: "one empty slice",
			s1:   []string{"a"},
			s2:   []string{},
			want: false,
		},
		{
			name: "equal slices, same order",
			s1:   []string{"a", "b", "c"},
			s2:   []string{"a", "b", "c"},
			want: true,
		},
		{
			name: "equal slices, different order",
			s1:   []string{"c", "a", "b"},
			s2:   []string{"a", "b", "c"},
			want: true,
		},
		{
			name: "different lengths",
			s1:   []string{"a", "b"},
			s2:   []string{"a", "b", "c"},
			want: false,
		},
		{
			name: "same length, different elements",
			s1:   []string{"a", "b", "d"},
			s2:   []string{"a", "b", "c"},
			want: false,
		},
		{
			name: "equal slices with duplicates",
			s1:   []string{"a", "b", "b", "c"},
			s2:   []string{"c", "b", "a", "b"},
			want: true,
		},
		{
			name: "different slices with duplicates",
			s1:   []string{"a", "b", "b", "c"},
			s2:   []string{"c", "b", "a", "a"},
			want: false,
		},
		{
			name: "nil slices",
			s1:   nil,
			s2:   nil,
			want: true,
		},
		{
			name: "one nil slice",
			s1:   []string{"a"},
			s2:   nil,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareStringSlice(tt.s1, tt.s2); got != tt.want {
				t.Errorf("compareStringSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}
