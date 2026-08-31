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
