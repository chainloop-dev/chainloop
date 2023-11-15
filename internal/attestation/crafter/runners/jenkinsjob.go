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

type JenkinsJob struct{}

const JenkinsJobID = "jenkins-job"

func NewJenkinsJob() *JenkinsJob {
	return &JenkinsJob{}
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

func (r *JenkinsJob) ListEnvVars() []string {
	return []string{
		// Some info about the job
		"JOB_NAME",
		"BUILD_URL",

		// Some info about the agent
		"AGENT_WORKDIR",
		"NODE_NAME",
	}
}

func (r *JenkinsJob) ListOptionalEnvVars() []string {
	return []string{
		// Some info about the branch
		"GIT_BRANCH",
		"GIT_COMMIT",
	}
}

func (r *JenkinsJob) ResolveEnvVars() map[string]string {
	return resolveEnvVars(r.ListEnvVars(), r.ListOptionalEnvVars())
}

func (r *JenkinsJob) String() string {
	return JenkinsJobID
}

func (r *JenkinsJob) RunURI() string {
	return os.Getenv("BUILD_URL")
}
