package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSessionIDsFromTrailer(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    []string
	}{
		{
			name:    "empty message",
			message: "",
			want:    nil,
		},
		{
			name:    "no trailer",
			message: "fix: typo\n\nA fix.\n",
			want:    nil,
		},
		{
			name:    "single id",
			message: "feat: x\n\nChainloop-Trace-Sessions: abc\n",
			want:    []string{"abc"},
		},
		{
			name:    "comma separated values are split, trimmed, and sorted",
			message: "feat: x\n\nChainloop-Trace-Sessions: foo, bar\n",
			want:    []string{"bar", "foo"},
		},
		{
			name:    "leading and trailing whitespace stripped",
			message: "feat: x\n\nChainloop-Trace-Sessions:   foo  ,  bar  \n",
			want:    []string{"bar", "foo"},
		},
		{
			name:    "empty value yields no IDs",
			message: "feat: x\n\nChainloop-Trace-Sessions:\n",
			want:    nil,
		},
		{
			name:    "extra commas are ignored",
			message: "feat: x\n\nChainloop-Trace-Sessions: foo,, ,bar\n",
			want:    []string{"bar", "foo"},
		},
		{
			name:    "duplicate ids are deduped",
			message: "feat: x\n\nChainloop-Trace-Sessions: a, a, b\n",
			want:    []string{"a", "b"},
		},
		{
			name:    "unrelated trailer ignored",
			message: "feat: x\n\nSigned-off-by: dev <dev@example.com>\n",
			want:    nil,
		},
		{
			name:    "case-sensitive key — lowercase is ignored",
			message: "feat: x\n\nchainloop-trace-sessions: foo\n",
			want:    nil,
		},
		{
			name:    "trailer line alongside other trailers",
			message: "feat: x\n\nBody.\n\nSigned-off-by: dev <dev@example.com>\nChainloop-Trace-Sessions: id-1, id-2\n",
			want:    []string{"id-1", "id-2"},
		},
		{
			name:    "leading whitespace before the trailer key is ignored",
			message: "feat: x\n\n  Chainloop-Trace-Sessions: foo\n",
			want:    nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseSessionIDsFromTrailer(tc.message)
			assert.Equal(t, tc.want, got)
		})
	}
}
