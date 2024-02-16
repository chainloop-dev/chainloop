//
// Copyright 2023 The Chainloop Authors.
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

package crafter

import (
	"errors"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/runners"
)

var ErrRunnerContextNotFound = errors.New("the runner environment doesn't match the required runner type")

type SupportedRunner interface {
	// Whether the attestation is happening in this environment
	CheckEnv() bool

	// List the env variables registered
	ListEnvVars() []*runners.EnvVarDefinition

	// Return the list of env vars associated with this runner already resolved
	ResolveEnvVars() (map[string]string, []*error)

	String() string

	// uri to the running job/workload
	RunURI() string

	// ID returns the runner type
	ID() schemaapi.CraftingSchema_Runner_RunnerType
}

type RunnerM map[schemaapi.CraftingSchema_Runner_RunnerType]SupportedRunner

var RunnersMap = map[schemaapi.CraftingSchema_Runner_RunnerType]SupportedRunner{
	schemaapi.CraftingSchema_Runner_GITHUB_ACTION:   runners.NewGithubAction(),
	schemaapi.CraftingSchema_Runner_GITLAB_PIPELINE: runners.NewGitlabPipeline(),
	schemaapi.CraftingSchema_Runner_AZURE_PIPELINE:  runners.NewAzurePipeline(),
	schemaapi.CraftingSchema_Runner_JENKINS_JOB:     runners.NewJenkinsJob(),
	schemaapi.CraftingSchema_Runner_CIRCLECI_BUILD:  runners.NewCircleCIBuild(),
	schemaapi.CraftingSchema_Runner_DAGGER_PIPELINE: runners.NewDaggerPipeline(),
}

// Load a specific runner
func NewRunner(t schemaapi.CraftingSchema_Runner_RunnerType) SupportedRunner {
	if r, ok := RunnersMap[t]; ok {
		return r
	}

	return runners.NewGeneric()
}

// Discover the runner environment
func DiscoverRunner() SupportedRunner {
	for _, r := range RunnersMap {
		if r.CheckEnv() {
			return r
		}
	}

	return runners.NewGeneric()
}
