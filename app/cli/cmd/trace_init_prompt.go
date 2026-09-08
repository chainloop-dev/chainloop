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
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"

	"charm.land/huh/v2"
)

// accessibleEnvVars turn on plain, numbered prompts instead of the full
// terminal UI, for screen readers and terminals that cannot render one.
// ACCESSIBLE is the convention the Charm tools use.
var accessibleEnvVars = []string{"CHAINLOOP_ACCESSIBLE", "ACCESSIBLE"}

// huhPrompter implements prompter against the terminal. It is deliberately
// thin: every decision lives in the resolvers in trace_init_identity.go, which
// are tested against a scripted fake instead of a terminal.
type huhPrompter struct {
	accessible bool
	// in and out are the streams the prompt uses. Prompts are drawn on stderr so
	// stdout stays clean for anything piping the command's output.
	in  io.Reader
	out io.Writer
}

// newHuhPrompter builds the terminal prompter.
func newHuhPrompter(lookupEnv func(string) (string, bool)) *huhPrompter {
	accessible := false
	for _, key := range accessibleEnvVars {
		if envEnabled(lookupEnv, key) {
			accessible = true
			break
		}
	}

	return &huhPrompter{accessible: accessible, in: os.Stdin, out: os.Stderr}
}

// Select asks the user to pick one of options, starting on defaultValue. Long
// lists can be narrowed by typing.
func (h *huhPrompter) Select(title string, options []string, defaultValue string) (string, error) {
	value := defaultValue

	// Filtering() is deliberately not set: it does not enable filtering, it puts
	// the field straight into filter-input mode, which replaces the title with an
	// empty "/" box and sends the arrow keys to the filter. Pressing "/" opens it
	// on demand, which is what the help line offers.
	field := huh.NewSelect[string]().
		Title(title).
		Options(huh.NewOptions(options...)...).
		Value(&value)

	if err := h.run(field); err != nil {
		return "", err
	}

	return value, nil
}

// MultiSelect asks the user to tick any number of options, with defaults
// already ticked. An empty submission is refused at the prompt, so the caller
// never has to send the user back through init to fix it.
func (h *huhPrompter) MultiSelect(title string, options, defaults []string) ([]string, error) {
	value := slices.Clone(defaults)

	field := huh.NewMultiSelect[string]().
		Title(title).
		Options(huh.NewOptions(options...)...).
		Value(&value).
		Validate(func(selected []string) error {
			if len(selected) == 0 {
				return errors.New("pick at least one, with space")
			}

			return nil
		})

	if err := h.run(field); err != nil {
		return nil, err
	}

	return value, nil
}

// Input asks the user to type a value, prefilled with defaultValue. The
// description line shows what a free-form answer will be normalized to, so the
// result is visible before it is submitted.
func (h *huhPrompter) Input(title, defaultValue string, validate func(string) error) (string, error) {
	value := defaultValue

	field := huh.NewInput().
		Title(title).
		Value(&value).
		Validate(validate).
		DescriptionFunc(func() string {
			slug := config.SlugifyDNS1123(value)
			if slug == "" || slug == value {
				return ""
			}

			return fmt.Sprintf("will be created as %q", slug)
		}, &value)

	if err := h.run(field); err != nil {
		return "", err
	}

	return value, nil
}

// run shows a single-field form and translates a dismissed prompt into
// errAborted, which trace init reports as a clean stop.
func (h *huhPrompter) run(field huh.Field) error {
	form := huh.NewForm(huh.NewGroup(field)).
		WithAccessible(h.accessible).
		WithInput(h.in).
		WithOutput(h.out)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errAborted
		}

		return err
	}

	return nil
}
