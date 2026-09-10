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

// Package prompt asks a user to pick from a list or type a value, on the
// terminal or in plain text where there is no terminal to draw on. It knows
// nothing about what is being asked for: callers pass the question, the answers
// and whatever validation belongs to their own domain.
package prompt

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"charm.land/huh/v2"
)

// ErrAborted is returned when the user dismisses a prompt. Dismissing a
// question is a decision rather than a failure, so callers usually stop
// cleanly on it instead of reporting an error.
var ErrAborted = errors.New("aborted")

// accessibleEnvVars turn on plain, numbered prompts instead of the full
// terminal UI, for screen readers and terminals that cannot render one.
// ACCESSIBLE is the convention the Charm tools use.
var accessibleEnvVars = []string{"CHAINLOOP_ACCESSIBLE", "ACCESSIBLE"}

// Prompter asks the questions. It is deliberately thin: it owns the terminal
// and nothing else, so the code deciding what to ask can be tested against a
// scripted fake rather than against a terminal.
type Prompter struct {
	accessible bool
	// in and out are the streams the prompt uses. Prompts are drawn on stderr so
	// stdout stays clean for anything piping the command's output.
	in  io.Reader
	out io.Writer
}

// New builds a prompter over the real terminal, in plain-text mode when the
// environment asks for it.
func New(lookupEnv func(string) (string, bool)) *Prompter {
	accessible := false
	for _, key := range accessibleEnvVars {
		if envEnabled(lookupEnv, key) {
			accessible = true
			break
		}
	}

	return &Prompter{accessible: accessible, in: os.Stdin, out: os.Stderr}
}

// NewForStreams builds a prompter over the given streams, in plain-text mode.
// It is how a test drives the real implementation without a terminal.
func NewForStreams(in io.Reader, out io.Writer) *Prompter {
	return &Prompter{accessible: true, in: in, out: out}
}

// Accessible reports whether prompts are drawn as plain numbered text.
func (p *Prompter) Accessible() bool {
	return p.accessible
}

// envEnabled reports whether an environment variable is set to something that
// means "yes". Present but empty, "0" or "false" is treated as unset.
func envEnabled(lookupEnv func(string) (string, bool), key string) bool {
	v, ok := lookupEnv(key)
	return ok && v != "" && v != "0" && v != "false"
}

// framed folds the description into the title when huh will not draw it.
// Accessible mode renders the title and the options and nothing else, so a
// description left in its own slot is simply lost there — and it carries the
// context the question needs, which is worth more than the layout.
func (p *Prompter) framed(title, description string) (string, string) {
	if description == "" || !p.accessible {
		return title, description
	}

	return fmt.Sprintf("%s (%s)", title, description), ""
}

// Select asks the user to pick one of options, starting on defaultValue. Long
// lists can be narrowed by typing. description is drawn under the title, inside
// the same frame, and validate — when set — refuses an answer and keeps the
// prompt open rather than returning it.
func (p *Prompter) Select(title, description string, options []string, defaultValue string, validate func(string) error) (string, error) {
	value := defaultValue
	title, description = p.framed(title, description)

	// Filtering() is deliberately not set: it does not enable filtering, it puts
	// the field straight into filter-input mode, which replaces the title with an
	// empty "/" box and sends the arrow keys to the filter. Pressing "/" opens it
	// on demand, which is what the help line offers.
	field := huh.NewSelect[string]().
		Title(title).
		Description(description).
		Options(huh.NewOptions(options...)...).
		Value(&value)

	// Set only when there is one: huh calls whatever it is handed, so a nil
	// validator panics rather than reading as "nothing to check".
	if validate != nil {
		field = field.Validate(validate)
	}

	if err := p.run(field); err != nil {
		return "", err
	}

	return value, nil
}

// MultiSelect asks the user to tick any number of options, with defaults
// already ticked. An empty submission is refused at the prompt, so the caller
// never has to send the user back to fix it.
func (p *Prompter) MultiSelect(title string, options, defaults []string) ([]string, error) {
	value := slices.Clone(defaults)

	if err := p.run(NewMultiSelectField(title, options, &value)); err != nil {
		return nil, err
	}

	return value, nil
}

// MultiSelectMaxVisible caps how many options are on screen at once, so a long
// list scrolls instead of pushing the prompt off the top. It is the height huh
// itself falls back to for dynamic options.
const MultiSelectMaxVisible = 10

// NewMultiSelectField builds the multi-select. It is separate from MultiSelect
// so a test can render it without a terminal to run a form against.
func NewMultiSelectField(title string, options []string, value *[]string) *huh.MultiSelect[string] {
	return huh.NewMultiSelect[string]().
		Title(title).
		Options(huh.NewOptions(options...)...).
		// The height has to be set. Left unset, huh sizes the viewport from the
		// options and then subtracts the title's line from that same number, so
		// the last option is silently cut off. Asking for one line more than the
		// options occupy cancels the subtraction out. Select does not share this
		// quirk, which is why only this field sets a height.
		Height(min(len(options), MultiSelectMaxVisible) + 1).
		Value(value).
		Validate(func(selected []string) error {
			if len(selected) == 0 {
				return errors.New("pick at least one, with space")
			}

			return nil
		})
}

// InputOpts is a question asking for a typed answer.
type InputOpts struct {
	Title string
	// Description is the line under the title, for context the question needs.
	Description string
	// Default prefills the field.
	Default string
	// Validate refuses an answer and keeps the prompt open.
	Validate func(string) error
	// Describe replaces Description as the answer is typed, for showing what an
	// answer will become before it is submitted. It is called with what has been
	// typed so far and returns the line to draw, which is usually Description
	// itself while there is nothing to add. Accessible mode draws no description
	// at all, so what it returns is not shown there.
	Describe func(value string) string
}

// Input asks the user to type a value.
func (p *Prompter) Input(opts InputOpts) (string, error) {
	value := opts.Default
	opts.Title, opts.Description = p.framed(opts.Title, opts.Description)

	if err := p.run(NewInputField(opts, &value)); err != nil {
		return "", err
	}

	return value, nil
}

// NewInputField builds the input. It is separate from Input so a test can
// render it without a terminal to run a form against, the same way
// NewMultiSelectField is.
func NewInputField(opts InputOpts, value *string) *huh.Input {
	field := huh.NewInput().
		Title(opts.Title).
		Description(opts.Description).
		Value(value)

	// Set only when there is one: huh calls whatever it is handed, so a nil
	// validator panics rather than reading as "nothing to check". Select has
	// the same trap.
	if opts.Validate != nil {
		field = field.Validate(opts.Validate)
	}

	if opts.Describe == nil {
		return field
	}

	// Both the static description and the function: DescriptionFunc is not
	// evaluated for the first draw, only once its binding changes, so on its own
	// it leaves an empty line where the context should be until the user types
	// something.
	return field.DescriptionFunc(func() string { return opts.Describe(*value) }, value)
}

// run shows a single-field form and translates a dismissed prompt into
// ErrAborted.
func (p *Prompter) run(field huh.Field) error {
	form := huh.NewForm(huh.NewGroup(field)).
		WithAccessible(p.accessible).
		WithInput(p.in).
		WithOutput(p.out)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return ErrAborted
		}

		return err
	}

	return nil
}
