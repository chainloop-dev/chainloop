//
// Copyright 2023-2026 The Chainloop Authors.
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
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/rs/zerolog"
)

var ErrRunnerContextNotFound = errors.New("the runner environment doesn't match the required runner type")

type SupportedRunner interface {
	// CheckEnv whether the attestation is happening in this environment
	CheckEnv() bool

	// ListEnvVars lists the env variables registered
	ListEnvVars() []*runners.EnvVarDefinition

	// ResolveEnvVars return the list of env vars associated with this runner already resolved
	ResolveEnvVars() (map[string]string, []*error)

	// RunURI to the running job/workload
	RunURI() string

	// ID returns the runner type
	ID() schemaapi.CraftingSchema_Runner_RunnerType

	// WorkflowFilePath returns the workflow file path associated with this runner
	WorkflowFilePath() string

	// IsAuthenticated returns whether the runner is authenticated or not
	IsAuthenticated() bool

	// Environment returns the runner environment
	Environment() runners.RunnerEnvironment

	// VerifyCommitSignature checks if a commit's signature is verified by the platform.
	// Returns nil if verification is not supported or not applicable for this runner.
	// Non-blocking: errors are logged and returned as unavailable status.
	VerifyCommitSignature(ctx context.Context, commitHash string) *commitverification.CommitVerification

	// Report writes attestation table output to platform-specific location.
	// attestationViewURL is an optional URL to view the attestation details.
	// Returns nil if platform doesn't support reporting.
	Report(tableOutput []byte, attestationViewURL string) error

	FederatedToken() string
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
	schemaapi.CraftingSchema_Runner_DAGGER_PIPELINE: func(authToken string, logger *zerolog.Logger) SupportedRunner {
		return runners.NewDaggerPipeline(authToken, logger)
	},
	schemaapi.CraftingSchema_Runner_TEAMCITY_PIPELINE: func(_ string, _ *zerolog.Logger) SupportedRunner {
		return runners.NewTeamCityPipeline()
	},
	schemaapi.CraftingSchema_Runner_TEKTON_PIPELINE: func(_ string, logger *zerolog.Logger) SupportedRunner {
		return runners.NewTektonPipeline(timeoutCtx, logger)
	},
	schemaapi.CraftingSchema_Runner_CHAINLOOP_SANDBOX: func(_ string, _ *zerolog.Logger) SupportedRunner {
		return runners.NewChainloopSandbox()
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
// by iterating over the different runners in discovery order and performing duck-typing checks.
// It returns the first matching runner immediately to avoid unnecessary CheckEnv()
// calls from remaining runners.
func DiscoverRunner(authToken string, logger zerolog.Logger) SupportedRunner {
	for _, runnerType := range runnerDiscoveryOrder() {
		r := RunnerFactories[runnerType](authToken, &logger)
		if r.CheckEnv() {
			return r
		}
	}

	return runners.NewGeneric()
}

// runnerDiscoveryOrder returns the runner types in the order DiscoverRunner checks them.
// The order must not depend on map iteration, because more than one runner can match the
// same environment. The Dagger runner goes first: the Chainloop Dagger module also passes
// the parent CI context (for example GITLAB_CI and CI_JOB_URL) to the CLI container, so
// the parent CI runner matches too. The other runners follow in enum order.
func runnerDiscoveryOrder() []schemaapi.CraftingSchema_Runner_RunnerType {
	order := make([]schemaapi.CraftingSchema_Runner_RunnerType, 0, len(RunnerFactories))
	for runnerType := range RunnerFactories {
		order = append(order, runnerType)
	}

	slices.SortFunc(order, func(a, b schemaapi.CraftingSchema_Runner_RunnerType) int {
		const first = schemaapi.CraftingSchema_Runner_DAGGER_PIPELINE
		switch {
		case a == first:
			return -1
		case b == first:
			return 1
		default:
			return cmp.Compare(a, b)
		}
	})

	return order
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
