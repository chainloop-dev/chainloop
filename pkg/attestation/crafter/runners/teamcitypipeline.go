//
// Copyright 2025 The Chainloop Authors.
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
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

type TeamCityPipeline struct{}

func NewTeamCityPipeline() *TeamCityPipeline {
	return &TeamCityPipeline{}
}

func (r *TeamCityPipeline) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_TEAMCITY_PIPELINE
}

// Checks whether we are within a TeamCity pipeline
func (r *TeamCityPipeline) CheckEnv() bool {
	return os.Getenv("TEAMCITY_PROJECT_NAME") != ""
}

func (r *TeamCityPipeline) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{
		{"BUILD_URL", false},
		{"TEAMCITY_PROJECT_NAME", false},
		{"TEAMCITY_VERSION", true},
		{"BUILD_NUMBER", true},
		{"USER", true},
		{"TEAMCITY_GIT_VERSION", true},
		{"BUILD_VCS_NUMBER", true},
		{"HOME", true},
	}
}

func (r *TeamCityPipeline) RunURI() string {
	return os.Getenv("BUILD_URL")
}

func (r *TeamCityPipeline) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}

func (r *TeamCityPipeline) WorkflowFilePath() string {
	return ""
}

func (r *TeamCityPipeline) IsAuthenticated() bool {
	return false
}

func (r *TeamCityPipeline) Environment() RunnerEnvironment {
	return Unknown
}
