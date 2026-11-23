//
// Copyright 2024-2026 The Chainloop Authors.
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

package runners

import (
	"fmt"
	"os"
	"slices"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/oidc"
)

type EnvVarDefinition struct {
	Name     string
	Optional bool
	// Identity marks a variable whose value identifies a person (a user name, login or email).
	// Organizations can redact these values with the "known identity" redaction mode.
	Identity bool
}

// KnownIdentityEnvVars returns the union of the env vars that any runner marks as Identity.
// The list does not depend on the runner type, so it is the same on the client and at push.
func KnownIdentityEnvVars() []string {
	allRunners := []interface{ ListEnvVars() []*EnvVarDefinition }{
		&AzurePipeline{}, &ChainloopSandbox{}, &CircleCIBuild{}, &DaggerPipeline{}, &Generic{},
		&GitHubAction{}, &GitlabPipeline{}, &JenkinsJob{}, &TeamCityPipeline{}, &TektonPipeline{},
	}

	var names []string
	for _, r := range allRunners {
		for _, def := range r.ListEnvVars() {
			if def.Identity {
				names = append(names, def.Name)
			}
		}
	}

	// Sorted and unique, so the CLI help, the docs and the init response are stable
	slices.Sort(names)
	return slices.Compact(names)
}

func resolveEnvVars(envVarsDefinitions []*EnvVarDefinition) (map[string]string, []*error) {
	result := make(map[string]string)
	var errors []*error

	for _, envVarDef := range envVarsDefinitions {
		value := os.Getenv(envVarDef.Name)
		if value != "" {
			result[envVarDef.Name] = value
			continue
		}

		if !envVarDef.Optional {
			err := fmt.Errorf("environment variable %s cannot be resolved", envVarDef.Name)
			errors = append(errors, &err)
		}
	}

	if len(errors) > 0 {
		result = nil
	}

	return result, errors
}

type RunnerEnvironment int

const (
	Managed RunnerEnvironment = iota
	SelfHosted
	Unknown
)

func (r RunnerEnvironment) String() string {
	switch r {
	case Managed:
		return "managed"
	case SelfHosted:
		return oidc.SelfHostedRunner
	}
	return "unknown"
}
