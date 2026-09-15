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

package aicodingsession

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveMode(t *testing.T) {
	testCases := []struct {
		name string
		mode string
		want string
	}{
		{name: "an unset mode is a coding session", mode: "", want: ModeCoding},
		{name: "an explicit coding mode is kept", mode: ModeCoding, want: ModeCoding},
		{name: "a generic mode is kept", mode: ModeGeneric, want: ModeGeneric},
		// Modes are not a closed set: one added after this binary was built
		// must pass through rather than collapse to the default.
		{name: "a mode this version predates is kept", mode: "spec", want: "spec"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ResolveMode(tc.mode))
		})
	}
}
