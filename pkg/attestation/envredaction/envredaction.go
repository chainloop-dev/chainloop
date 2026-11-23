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

// Package envredaction replaces the values of selected environment variables stored in an
// attestation with a fixed placeholder. The crafter applies it, and the control plane uses
// the same rules to check pushed attestations.
package envredaction

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners"
)

// RedactedValue replaces the value of a redacted environment variable.
const RedactedValue = "[REDACTED]"

// Mode selects which environment variables are redacted.
type Mode string

const (
	// ModeNone disables redaction.
	ModeNone Mode = "none"
	// ModeKnownIdentity redacts the variables that runner integrations mark as identifying a person.
	ModeKnownIdentity Mode = "known-identity"
	// ModeCustom redacts only an explicit list of variables.
	ModeCustom Mode = "custom"
)

// Names returns the exact variable names that the mode redacts.
// custom is used only by ModeCustom, which requires at least one non-empty name.
func Names(mode Mode, custom []string) ([]string, error) {
	switch mode {
	case ModeNone, "":
		return nil, nil
	case ModeKnownIdentity:
		return runners.KnownIdentityEnvVars(), nil
	case ModeCustom:
		if len(custom) == 0 {
			return nil, errors.New("custom env var redaction requires at least one variable name")
		}
		for _, name := range custom {
			if strings.TrimSpace(name) == "" {
				return nil, errors.New("custom env var redaction contains an empty variable name")
			}
			// an exact name with surrounding whitespace never matches a variable, so its value stays clear
			if strings.TrimSpace(name) != name {
				return nil, fmt.Errorf("custom env var redaction name %q has leading or trailing whitespace", name)
			}
		}
		return custom, nil
	default:
		return nil, fmt.Errorf("invalid env var redaction mode %q, valid values are: %s, %s, %s", mode, ModeNone, ModeKnownIdentity, ModeCustom)
	}
}

// Apply replaces, in place, the value of each named variable that is present in env.
// The key stays, so the attestation shows that a value existed and was hidden on purpose.
func Apply(env map[string]string, names []string) {
	for _, name := range names {
		if _, ok := env[name]; ok {
			env[name] = RedactedValue
		}
	}
}

// Violations returns, sorted, the named variables that have a non-empty value other than
// RedactedValue. An absent or empty value reveals nothing.
func Violations(env map[string]string, names []string) []string {
	var out []string
	for _, name := range names {
		if v := env[name]; v != "" && v != RedactedValue {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}
