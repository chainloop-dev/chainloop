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

package policies

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeRuntimeInputs(t *testing.T) {
	testCases := []struct {
		name          string
		with          map[string]string
		runtimeInputs map[string]string
		want          map[string]string
	}{
		{
			name:          "no runtime inputs returns contract args unchanged",
			with:          map[string]string{"ignored_paths": "a,b"},
			runtimeInputs: nil,
			want:          map[string]string{"ignored_paths": "a,b"},
		},
		{
			name:          "runtime input on empty contract key",
			with:          map[string]string{},
			runtimeInputs: map[string]string{"ignored_paths": "c\nd"},
			want:          map[string]string{"ignored_paths": "c\nd"},
		},
		{
			name:          "runtime input merges additively with contract value",
			with:          map[string]string{"ignored_paths": "a,b"},
			runtimeInputs: map[string]string{"ignored_paths": "c\nd"},
			want:          map[string]string{"ignored_paths": "a,b\nc\nd"},
		},
		{
			name:          "runtime input on a different key is added alongside",
			with:          map[string]string{"paths": "**"},
			runtimeInputs: map[string]string{"ignored_paths": "c"},
			want:          map[string]string{"paths": "**", "ignored_paths": "c"},
		},
		{
			name:          "empty contract value is replaced, not prefixed with newline",
			with:          map[string]string{"ignored_paths": ""},
			runtimeInputs: map[string]string{"ignored_paths": "c"},
			want:          map[string]string{"ignored_paths": "c"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeRuntimeInputs(tc.with, tc.runtimeInputs)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestMergeRuntimeInputsDoesNotMutate ensures the input maps are left untouched.
func TestMergeRuntimeInputsDoesNotMutate(t *testing.T) {
	with := map[string]string{"ignored_paths": "a"}
	runtimeInputs := map[string]string{"ignored_paths": "b"}

	_ = mergeRuntimeInputs(with, runtimeInputs)

	assert.Equal(t, map[string]string{"ignored_paths": "a"}, with)
	assert.Equal(t, map[string]string{"ignored_paths": "b"}, runtimeInputs)
}
