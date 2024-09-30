//
// Copyright 2024 The Chainloop Authors.
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

type JenkinsJob struct{}

func NewJenkinsJob() *JenkinsJob {
	return &JenkinsJob{}
}

func (r *JenkinsJob) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_JENKINS_JOB
}

// Checks whether we are within a Jenkins job
func (r *JenkinsJob) CheckEnv() bool {
	for _, envVarName := range []string{"JENKINS_HOME", "BUILD_URL"} {
		if os.Getenv(envVarName) == "" {
			return false
		}
	}

	return true
}

func (r *JenkinsJob) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{
		// Some info about the job
		{"JOB_NAME", false},
		{"BUILD_URL", false},

		// Some info about the commit (Jenkins Git Plugin)
		// NOTE: These variables are marked as optional because their presence
		//       depends the jenkins' configuration (e.g., multibranch pipelines).
		{"GIT_BRANCH", true},
		{"GIT_COMMIT", true},

		// Some info about the agent
		// We've found this one to be optional
		{"AGENT_WORKDIR", true},
		// Workspace is required as long as the jobs run inside a `node` block
		{"WORKSPACE", false},
		{"NODE_NAME", false},
	}
}

func (r *JenkinsJob) RunURI() string {
	return os.Getenv("BUILD_URL")
}

func (r *JenkinsJob) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}
