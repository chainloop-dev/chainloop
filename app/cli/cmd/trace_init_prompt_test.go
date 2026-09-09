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
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/huh/v2"
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

	got, err := p.Select("Pick one", []string{"alpha", "beta", "gamma"}, "alpha", nil)
	require.NoError(t, err)
	assert.Equal(t, "beta", got)
	assert.Contains(t, out.String(), "Pick one")
}

// A validator lets the picker show an option it will not accept, which is how
// an organization the caller can only view stays visible without being usable.
func TestHuhPrompterSelectValidate(t *testing.T) {
	// The refused option first, then an acceptable one: accessible mode asks
	// again rather than giving up.
	p, out := accessiblePrompter("2\n1\n")

	got, err := p.Select("Pick one", []string{"alpha", "beta"}, "alpha", func(chosen string) error {
		if chosen == "beta" {
			return errors.New("beta is not available")
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "alpha", got)
	assert.Contains(t, out.String(), "beta is not available")
}

func TestHuhPrompterInput(t *testing.T) {
	p, out := accessiblePrompter("my-project\n")

	got, err := p.Input("Project name", "default-name", newProjectValidator(nil))
	require.NoError(t, err)
	assert.Equal(t, "my-project", got)
	assert.Contains(t, out.String(), "Project name")
}

func TestHuhPrompterMultiSelect(t *testing.T) {
	// Accessible mode toggles an option by its number and finishes on 0, so
	// this ticks the second option on top of the preselected first one.
	p, out := accessiblePrompter("2\n0\n")

	// The prompter is generic, so the options here are deliberately not provider
	// names: nothing about it should depend on what is being picked.
	got, err := p.MultiSelect("Pick options", []string{"first", "second", "third"}, []string{"first"})
	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, got)
	assert.Contains(t, out.String(), "Pick options")
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

// TestMultiSelectFieldShowsEveryOption renders the real field and looks for
// every option in the output. Left to size itself, huh's multi-select subtracts
// the title's line from the height it derived from the options, so the last one
// never appears and the user cannot tell there is more to tick.
func TestMultiSelectFieldShowsEveryOption(t *testing.T) {
	for _, count := range []int{1, 2, 3, 5, multiSelectMaxVisible} {
		t.Run(fmt.Sprintf("%d options", count), func(t *testing.T) {
			options := make([]string, 0, count)
			for i := range count {
				options = append(options, fmt.Sprintf("option-%d", i))
			}

			var value []string
			form := huh.NewForm(huh.NewGroup(newMultiSelectField("Pick some", options, &value)))
			// Init sizes the field, which is what this is about. Its command
			// starts the cursor blinking and is nothing to run here.
			_ = form.Init()

			view := form.View()
			for _, o := range options {
				assert.Contains(t, view, o, "every option must be on screen")
			}
		})
	}
}
