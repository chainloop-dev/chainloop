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

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/cursor"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/opencode"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"
)

const providersPromptTitle = "Agents to trace in this repository"

// traceProviderFlags records which --claude / --cursor / --opencode flags the
// user set.
type traceProviderFlags struct {
	claude, cursor, opencode bool
}

// any reports whether the user named at least one provider.
func (f traceProviderFlags) any() bool {
	return f.claude || f.cursor || f.opencode
}

// names returns the providers the flags select, in the registry's order rather
// than the order the flags happen to appear in.
func (f traceProviderFlags) names() []string {
	out := make([]string, 0, 3)
	if f.claude {
		out = append(out, claude.Name)
	}
	if f.cursor {
		out = append(out, cursor.Name)
	}
	if f.opencode {
		out = append(out, opencode.Name)
	}

	return out
}

// namesOrDefault returns the providers the flags select, falling back to the
// default when none was named. `trace run` is flag-driven and never prompts,
// so it resolves its providers this way.
func (f traceProviderFlags) namesOrDefault() []string {
	if !f.any() {
		return []string{providers.DefaultProvider}
	}

	return f.names()
}

// traceProviderNames lists every registered provider, in the order the registry
// declares them.
func traceProviderNames() []string {
	all := providers.All()

	names := make([]string, 0, len(all))
	for _, p := range all {
		names = append(names, p.Name())
	}

	return names
}

// resolveTraceProviders picks the agents whose hooks init installs. Flags win
// and skip the question, the same way --org and --project do. Otherwise a
// terminal session ticks them off a list with the default preselected, and a
// non-interactive one keeps that default so scripted runs are unchanged.
func resolveTraceProviders(p prompter, flags traceProviderFlags, interactive bool) ([]string, error) {
	if flags.any() {
		return flags.names(), nil
	}

	if !interactive {
		return []string{providers.DefaultProvider}, nil
	}

	chosen, err := p.MultiSelect(providersPromptTitle, traceProviderNames(), []string{providers.DefaultProvider})
	if err != nil {
		return nil, err
	}

	// The prompt refuses an empty submission, so this only catches a prompter
	// that does not, but tracing nothing would install hooks that never fire.
	if len(chosen) == 0 {
		return nil, errors.New("select at least one agent to trace")
	}

	return chosen, nil
}
