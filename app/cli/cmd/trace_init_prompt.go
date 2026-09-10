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
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/prompt"
)

// namingPrompter is the prompt package plus the one thing about these questions
// that is Chainloop's rather than the terminal's: an answer typed freely is
// stored as a DNS-1123 label, so the input shows what it will become. Nothing
// else here knows about naming rules.
type namingPrompter struct {
	*prompt.Prompter
}

// newHuhPrompter builds the prompter `trace init` asks its questions through.
func newHuhPrompter(lookupEnv func(string) (string, bool)) namingPrompter {
	return namingPrompter{prompt.New(lookupEnv)}
}

// Input asks for a name, showing the label it will be stored as once the two
// differ. Seeing what will be created matters more at that point than the
// context it will be created in, so it takes the description's place.
func (n namingPrompter) Input(title, description, defaultValue string, validate func(string) error) (string, error) {
	return n.Prompter.Input(prompt.InputOpts{
		Title:       title,
		Description: description,
		Default:     defaultValue,
		Validate:    validate,
		Describe: func(value string) string {
			return nameDescription(description, value)
		},
	})
}

// nameDescription is the line under a name prompt's title: the standing
// context, giving way to what the answer normalizes to once the answer and its
// label differ.
func nameDescription(description, value string) string {
	slug := config.SlugifyDNS1123(value)
	if slug == "" || slug == value {
		return description
	}

	return fmt.Sprintf("will be created as %q", slug)
}
