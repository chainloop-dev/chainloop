package attribution

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChangedPaths(t *testing.T) {
	t.Run("detects created, modified, and deleted paths", func(t *testing.T) {
		before := map[string]string{"a": "1", "b": "2", "c": "3"}
		after := map[string]string{"a": "1", "b": "9", "d": "4"}

		changed, deleted := ChangedPaths(before, after)

		assert.Equal(t, []string{"b", "d"}, changed) // b modified, d created
		assert.Equal(t, []string{"c"}, deleted)      // c removed
	})

	t.Run("empty when signatures are equal", func(t *testing.T) {
		sig := map[string]string{"a": "1", "b": "2"}

		changed, deleted := ChangedPaths(sig, sig)

		assert.Empty(t, changed)
		assert.Empty(t, deleted)
	})

	t.Run("all created from empty before", func(t *testing.T) {
		after := map[string]string{"b": "2", "a": "1"}

		changed, deleted := ChangedPaths(nil, after)

		assert.Equal(t, []string{"a", "b"}, changed) // sorted
		assert.Empty(t, deleted)
	})
}
