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
