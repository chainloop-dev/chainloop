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
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accessiblePrompter drives the real huh implementation in accessible mode,
// where prompts are plain text over the given streams. It is the only way to
// exercise the terminal code without a terminal.
func accessiblePrompter(input string) (*huhPrompter, *bytes.Buffer) {
	out := &bytes.Buffer{}

	return &huhPrompter{accessible: true, in: strings.NewReader(input), out: out}, out
}

func TestHuhPrompterSelect(t *testing.T) {
	// Accessible mode numbers the options and reads the choice as a line.
	p, out := accessiblePrompter("2\n")

	got, err := p.Select("Pick one", []string{"alpha", "beta", "gamma"}, "alpha")
	require.NoError(t, err)
	assert.Equal(t, "beta", got)
	assert.Contains(t, out.String(), "Pick one")
}

func TestHuhPrompterInput(t *testing.T) {
	p, out := accessiblePrompter("my-project\n")

	got, err := p.Input("Project name", "default-name", validateNewProjectName)
	require.NoError(t, err)
	assert.Equal(t, "my-project", got)
	assert.Contains(t, out.String(), "Project name")
}

func TestNewHuhPrompterAccessibility(t *testing.T) {
	testCases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{name: "off by default", want: false},
		{name: "the Chainloop variable turns it on", env: map[string]string{"CHAINLOOP_ACCESSIBLE": "1"}, want: true},
		{name: "the Charm convention turns it on", env: map[string]string{"ACCESSIBLE": "true"}, want: true},
		{name: "set to 0 leaves it off", env: map[string]string{"ACCESSIBLE": "0"}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := newHuhPrompter(func(key string) (string, bool) {
				v, ok := tc.env[key]
				return v, ok
			})
			assert.Equal(t, tc.want, p.accessible)
		})
	}
}
