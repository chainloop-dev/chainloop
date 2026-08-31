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

package attribution

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/stretchr/testify/require"
)

func TestReverseApplyEdits(t *testing.T) {
	tests := []struct {
		name    string
		after   string
		edits   []trace.HookEdit
		want    string // expected reconstructed content when wantNil is false
		wantNil bool   // explicit "result must be nil"; distinct from a legitimate empty result
	}{
		{
			name:    "no edits",
			after:   "hello",
			edits:   nil,
			wantNil: true,
		},
		{
			name:  "single replace",
			after: "hello world",
			edits: []trace.HookEdit{{OldString: "there", NewString: "world"}},
			want:  "hello there",
		},
		{
			name:  "multiple sequential edits reversed in order",
			after: "foo BAR baz QUUX",
			edits: []trace.HookEdit{
				{OldString: "bar", NewString: "BAR"},
				{OldString: "quux", NewString: "QUUX"},
			},
			want: "foo bar baz quux",
		},
		{
			name:  "missing new_string is skipped",
			after: "hello world",
			edits: []trace.HookEdit{
				{OldString: "xyz", NewString: "not present"},
				{OldString: "there", NewString: "world"},
			},
			want: "hello there",
		},
		{
			name:  "insertion (empty old) is reversed to deletion",
			after: "prefix-content",
			edits: []trace.HookEdit{{OldString: "", NewString: "prefix-"}},
			want:  "content",
		},
		{
			name:    "deletion (empty new) skipped because new_string empty",
			after:   "abc",
			edits:   []trace.HookEdit{{OldString: "removed", NewString: ""}},
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ReverseApplyEdits([]byte(tc.after), tc.edits)
			if tc.wantNil {
				require.Nil(t, got)

				return
			}
			require.Equal(t, tc.want, string(got))
		})
	}
}
