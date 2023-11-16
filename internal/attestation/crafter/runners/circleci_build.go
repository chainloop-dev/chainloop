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

package runners

import "os"

type CircleCIBuild struct{}

const CircleCIBuildID = "circleci-build"

func NewCircleCIBuild() *CircleCIBuild {
	return &CircleCIBuild{}
}

func (r *CircleCIBuild) CheckEnv() bool {
	for _, envVarName := range []string{"CI", "CIRCLECI"} {
		if os.Getenv(envVarName) != "true" {
			return false
		}
	}

	return true
}

func (r *CircleCIBuild) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{
		// Some info about the job
		{"CIRCLE_BUILD_URL", false},
		{"CIRCLE_JOB", false},

		// Some info about the commit
		{"CIRCLE_BRANCH", false},

		// Some info about the agent
		{"CIRCLE_NODE_TOTAL", false},
		{"CIRCLE_NODE_INDEX", false},
	}
}

func (r *CircleCIBuild) String() string {
	return JenkinsJobID
}

func (r *CircleCIBuild) RunURI() string {
	return os.Getenv("CIRCLE_BUILD_URL")
}

func (r *CircleCIBuild) ResolveEnvVars() (map[string]string, error) {
	return resolveEnvVars(r.ListEnvVars())
}
