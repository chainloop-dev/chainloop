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

package e2e

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodexViewerMaterialPointer(t *testing.T) {
	for _, tc := range []struct {
		name, line string
		want       bool
	}{
		{name: "metadata alone", line: `{"message":{"content":"Hello"},"source_event":{"type":"chainloop.replaced","digest":"sha256:spec"}}`},
		{name: "tool result", line: `{"message":{"content":[{"type":"tool_result","content":[{"type":"chainloop.replaced","digest":"sha256:spec"}]}]}}`, want: true},
		{name: "image source", line: `{"message":{"content":[{"type":"image","source":{"type":"chainloop.replaced","digest":"sha256:spec"}}]}}`, want: true},
		{name: "wrong material", line: `{"message":{"content":[{"type":"chainloop.replaced","digest":"sha256:other"}]}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, codexViewerPointer([]json.RawMessage{json.RawMessage(tc.line)}, "sha256:spec"))
		})
	}
}
