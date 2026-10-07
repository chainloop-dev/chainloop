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

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/cursor"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/opencode"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"
)

const providersPromptTitle = "Harnesses to trace in this repository"

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

// configuredTraceProviders lists the harnesses whose hooks the repository
// already carries, in the registry's order, so a re-run of init offers what an
// earlier one set up. A harness whose configuration cannot be read counts as
// not configured: init is how it gets repaired, so it must not stop init.
func configuredTraceProviders(repoRoot string) []string {
	var configured []trace.Provider
	for _, p := range providers.All() {
		installed, err := p.HooksInstalled(repoRoot)
		if err != nil {
			logger.Debug().Err(err).Str("harness", p.Name()).Msg("could not read the harness configuration")
			continue
		}

		if installed {
			configured = append(configured, p)
		}
	}

	return providerNames(configured)
}

// resolveTraceProviders picks the agents whose hooks init installs. Flags win
// and skip the question, the same way --org and --project do. Otherwise a
// terminal session ticks them off a list with the harnesses already configured
// preselected, or the default on a first run, and a non-interactive one keeps
// the default so scripted runs are unchanged. configured is only called when
// the question is asked, so the other paths read nothing from the repository.
func resolveTraceProviders(p prompter, flags traceProviderFlags, interactive bool, configured func() []string) ([]string, error) {
	if flags.any() {
		return flags.names(), nil
	}

	if !interactive {
		return []string{providers.DefaultProvider}, nil
	}

	defaults := configured()
	if len(defaults) == 0 {
		defaults = []string{providers.DefaultProvider}
	}

	chosen, err := p.MultiSelect(providersPromptTitle, traceProviderNames(), defaults)
	if err != nil {
		return nil, err
	}

	// The prompt refuses an empty submission, so this only catches a prompter
	// that does not, but tracing nothing would install hooks that never fire.
	if len(chosen) == 0 {
		return nil, errors.New("select at least one harness to trace")
	}

	return chosen, nil
}
