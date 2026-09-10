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

package prompt

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

// accessiblePrompter drives the real implementation in accessible mode, where
// prompts are plain text over the given streams. It is the only way to exercise
// the terminal code without a terminal.
func accessiblePrompter(input string) (*Prompter, *bytes.Buffer) {
	out := &bytes.Buffer{}

	return NewForStreams(strings.NewReader(input), out), out
}

// The options the Select tests pick between, in the order they are offered.
const (
	optionAlpha = "alpha"
	optionBeta  = "beta"
	optionGamma = "gamma"
)

func TestSelect(t *testing.T) {
	// Accessible mode numbers the options and reads the choice as a line.
	p, out := accessiblePrompter("2\n")

	got, err := p.Select("Pick one", "in some context", []string{optionAlpha, optionBeta, optionGamma}, optionAlpha, nil)
	require.NoError(t, err)
	assert.Equal(t, optionBeta, got)
	assert.Contains(t, out.String(), "Pick one")
}

// A validator lets a picker show an option it will not accept, which is how an
// entry can stay visible without being usable.
func TestSelectValidate(t *testing.T) {
	// The refused option first, then an acceptable one: accessible mode asks
	// again rather than giving up.
	p, out := accessiblePrompter("2\n1\n")

	const refusal = "beta is not available"

	got, err := p.Select("Pick one", "", []string{optionAlpha, optionBeta}, optionAlpha, func(chosen string) error {
		if chosen == optionBeta {
			return errors.New(refusal)
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, optionAlpha, got)
	assert.Contains(t, out.String(), refusal)
}

func TestInput(t *testing.T) {
	p, out := accessiblePrompter("typed-value\n")

	// No Validate and no Describe on purpose: huh panics on a nil validator, so
	// a question that checks nothing has to reach the prompt all the same.
	got, err := p.Input(InputOpts{Title: "A name", Description: "in some context", Default: "a-default"})
	require.NoError(t, err)
	assert.Equal(t, "typed-value", got)
	assert.Contains(t, out.String(), "A name")
}

func TestMultiSelect(t *testing.T) {
	// Accessible mode toggles an option by its number and finishes on 0, so
	// this ticks the second option on top of the preselected first one.
	p, out := accessiblePrompter("2\n0\n")

	const first, second = "first", "second"

	got, err := p.MultiSelect("Pick options", []string{first, second, "third"}, []string{first})
	require.NoError(t, err)
	assert.Equal(t, []string{first, second}, got)
	assert.Contains(t, out.String(), "Pick options")
}

func TestNewAccessibility(t *testing.T) {
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
			p := New(func(key string) (string, bool) {
				v, ok := tc.env[key]
				return v, ok
			})
			assert.Equal(t, tc.want, p.Accessible())
		})
	}
}

// Accessible mode draws the title and the options and nothing else, so context
// left in the description slot would be lost to a screen reader.
func TestAccessibleFoldsTheDescriptionIn(t *testing.T) {
	p, out := accessiblePrompter("1\n")

	_, err := p.Select("Pick one", "some context", []string{optionAlpha}, optionAlpha, nil)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Pick one (some context)")
}

// TestMultiSelectFieldShowsEveryOption renders the real field and looks for
// every option in the output. Left to size itself, huh's multi-select subtracts
// the title's line from the height it derived from the options, so the last one
// never appears and the user cannot tell there is more to tick.
func TestMultiSelectFieldShowsEveryOption(t *testing.T) {
	for _, count := range []int{1, 2, 3, 5, MultiSelectMaxVisible} {
		t.Run(fmt.Sprintf("%d options", count), func(t *testing.T) {
			options := make([]string, 0, count)
			for i := range count {
				options = append(options, fmt.Sprintf("option-%d", i))
			}

			var value []string
			form := huh.NewForm(huh.NewGroup(NewMultiSelectField("Pick some", options, &value)))
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

// An input's description has to be there on the first draw, before anything is
// typed: huh does not evaluate DescriptionFunc until its binding changes, so a
// field relying on that alone opens with an empty line where the context goes.
func TestInputFieldShowsItsDescriptionBeforeTyping(t *testing.T) {
	const context = "some context"

	value := ""
	form := huh.NewForm(huh.NewGroup(NewInputField(InputOpts{
		Title:       "A name",
		Description: context,
		Describe:    func(string) string { return "only once something is typed" },
	}, &value)))
	_ = form.Init()

	assert.Contains(t, form.View(), context, "the context must be on screen before anything is typed")
}
