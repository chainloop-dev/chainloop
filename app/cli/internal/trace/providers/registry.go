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

// Package providers registers all available AI coding agent trace providers
// in one place. Callers (CLI commands, pre-push hook) use this registry
// instead of instantiating provider packages directly, so adding a new
// provider is a one-line change here.
package providers

import (
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/cursor"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/opencode"
)

// DefaultProvider is the provider name used when none is specified
// (e.g. `trace init` with no --claude/--cursor flag, or a SessionRecord
// predating the Provider field).
const DefaultProvider = trace.DefaultProviderName

// all holds the canonical set of providers. Instances are allocated once at
// init so callers don't pay for re-construction on every lookup.
var all = []trace.Provider{
	claude.New(),
	cursor.New(),
	opencode.New(),
}

// byName is a lookup-optimized view of all, built once at init.
var byName = func() map[string]trace.Provider {
	m := make(map[string]trace.Provider, len(all))
	for _, p := range all {
		m[p.Name()] = p
	}

	return m
}()

// All returns every registered provider in a stable, deterministic order.
// The order is meaningful for user-facing listings in `trace init` logs.
func All() []trace.Provider {
	return all
}

// ByName returns the registered provider with the given name, or nil when
// the name is unknown.
func ByName(name string) trace.Provider {
	return byName[name]
}

// ByNames resolves a list of names into providers, dropping any unknown
// entries. Returns an empty slice (not nil) when no names match, so
// callers can range without nil checks.
func ByNames(names []string) []trace.Provider {
	out := make([]trace.Provider, 0, len(names))
	for _, n := range names {
		if p := byName[n]; p != nil {
			out = append(out, p)
		}
	}

	return out
}
