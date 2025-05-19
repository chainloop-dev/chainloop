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
	"context"
	"errors"
	"fmt"
	"time"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners"
	"github.com/rs/zerolog"
)

var ErrRunnerContextNotFound = errors.New("the runner environment doesn't match the required runner type")

type SupportedRunner interface {
	// Whether the attestation is happening in this environment
	CheckEnv() bool

	// List the env variables registered
	ListEnvVars() []*runners.EnvVarDefinition

	// Return the list of env vars associated with this runner already resolved
	ResolveEnvVars() (map[string]string, []*error)

	// uri to the running job/workload
	RunURI() string

	// ID returns the runner type
	ID() schemaapi.CraftingSchema_Runner_RunnerType

	// WorkflowFilePath returns the workflow file path associated with this runner
	WorkflowFilePath() string

	// IsAuthenticated returns whether the runner is authenticated or not
	IsAuthenticated() bool

	// RunnerEnvironment returns the runner environment
	Environment() runners.RunnerEnvironment
}

type RunnerM map[schemaapi.CraftingSchema_Runner_RunnerType]SupportedRunner

// timeoutCtx is a context with a 15-second timeout
var timeoutCtx, _ = context.WithTimeout(context.Background(), 15*time.Second)

// RunnerFactory is a function that creates a runner
type RunnerFactory func(authToken string, logger *zerolog.Logger) SupportedRunner

// RunnerFactories maps runner types to factory functions that create them
var RunnerFactories = map[schemaapi.CraftingSchema_Runner_RunnerType]RunnerFactory{
	schemaapi.CraftingSchema_Runner_GITHUB_ACTION: func(_ string, logger *zerolog.Logger) SupportedRunner {
		return runners.NewGithubAction(timeoutCtx, logger)
	},
	schemaapi.CraftingSchema_Runner_GITLAB_PIPELINE: func(authToken string, logger *zerolog.Logger) SupportedRunner {
		return runners.NewGitlabPipeline(timeoutCtx, authToken, logger)
	},
	schemaapi.CraftingSchema_Runner_AZURE_PIPELINE: func(_ string, _ *zerolog.Logger) SupportedRunner {
		return runners.NewAzurePipeline()
	},
	schemaapi.CraftingSchema_Runner_JENKINS_JOB: func(_ string, _ *zerolog.Logger) SupportedRunner {
		return runners.NewJenkinsJob()
	},
	schemaapi.CraftingSchema_Runner_CIRCLECI_BUILD: func(_ string, _ *zerolog.Logger) SupportedRunner {
		return runners.NewCircleCIBuild()
	},
	schemaapi.CraftingSchema_Runner_DAGGER_PIPELINE: func(_ string, _ *zerolog.Logger) SupportedRunner {
		return runners.NewDaggerPipeline()
	},
	schemaapi.CraftingSchema_Runner_TEAMCITY_PIPELINE: func(_ string, _ *zerolog.Logger) SupportedRunner {
		return runners.NewTeamCityPipeline()
	},
}

// Load a specific runner
func NewRunner(t schemaapi.CraftingSchema_Runner_RunnerType, authToken string, logger *zerolog.Logger) SupportedRunner {
	if factory, ok := RunnerFactories[t]; ok {
		return factory(authToken, logger)
	}

	return runners.NewGeneric()
}

// DiscoverRunner the runner environment
// This method does a simple check to see which runner is available in the environment
// by iterating over the different runners and performing duck-typing checks
// If more than one runner is detected, we default to generic since its an incongruent result
func DiscoverRunner(authToken string, logger zerolog.Logger) SupportedRunner {
	detected := []SupportedRunner{}

	// Create all runners and check their environment
	for _, factory := range RunnerFactories {
		r := factory(authToken, &logger)
		if r.CheckEnv() {
			detected = append(detected, r)
		}
	}

	// if we don't detect any runner or more than one, we default to generic
	if len(detected) == 0 {
		return runners.NewGeneric()
	}

	if len(detected) > 1 {
		var detectedStr []string
		for _, d := range detected {
			detectedStr = append(detectedStr, d.ID().String())
		}

		logger.Warn().Strs("detected", detectedStr).Msg("multiple runners detected, incongruent environment")
		return runners.NewGeneric()
	}

	return detected[0]
}

func DiscoverAndEnforceRunner(enforcedRunnerType schemaapi.CraftingSchema_Runner_RunnerType, dryRun bool, authToken string, logger zerolog.Logger) (SupportedRunner, error) {
	discoveredRunner := DiscoverRunner(authToken, logger)

	logger.Debug().
		Str("discovered", discoveredRunner.ID().String()).
		Str("enforced", enforcedRunnerType.String()).
		Msg("checking runner context")

	// If the runner type is not specified and it's a dry run, we don't enforce it
	if enforcedRunnerType == schemaapi.CraftingSchema_Runner_RUNNER_TYPE_UNSPECIFIED || dryRun {
		return discoveredRunner, nil
	}

	// Otherwise we enforce the runner type
	if enforcedRunnerType != discoveredRunner.ID() {
		return nil, fmt.Errorf("runner not found %s", enforcedRunnerType)
	}

	return discoveredRunner, nil
}
