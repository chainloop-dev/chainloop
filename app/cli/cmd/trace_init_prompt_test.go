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

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNameDescription covers the line under a name prompt's title, which is the
// only thing about these prompts that is Chainloop's rather than the terminal's.
// The prompt package covers the rest.
func TestNameDescription(t *testing.T) {
	const context = "you do not belong to one yet"

	testCases := []struct {
		name        string
		description string
		value       string
		want        string
	}{
		{name: "an empty answer keeps the context", description: context, want: context},
		{
			name:        "an answer that needs no normalizing keeps it",
			description: context, value: "my-project", want: context,
		},
		{
			name:        "an answer that normalizes shows what it becomes",
			description: context, value: "My Project", want: `will be created as "my-project"`,
		},
		{
			// Nothing usable to show yet, so the context stands rather than
			// flickering to an empty line.
			name:        "an answer with nothing usable in it keeps the context",
			description: context, value: nameWithNothingUsable, want: context,
		},
		{name: "no context and no answer shows nothing", value: "my-project"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, nameDescription(tc.description, tc.value))
		})
	}
}
