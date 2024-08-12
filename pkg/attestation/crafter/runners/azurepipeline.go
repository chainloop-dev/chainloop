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

import (
	neturl "net/url"
	"os"
	"path"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

type AzurePipeline struct{}

func NewAzurePipeline() *AzurePipeline {
	return &AzurePipeline{}
}

func (r *AzurePipeline) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_AZURE_PIPELINE
}

// Figure out if we are in a Azure Pipeline job or not
func (r *AzurePipeline) CheckEnv() bool {
	for _, varName := range []string{"TF_BUILD", "BUILD_BUILDURI"} {
		if os.Getenv(varName) == "" {
			return false
		}
	}

	return true
}

func (r *AzurePipeline) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{
		{"BUILD_REQUESTEDFOREMAIL", false},
		{"BUILD_REQUESTEDFOR", false},
		{"BUILD_REPOSITORY_URI", false},
		{"BUILD_REPOSITORY_NAME", false},
		{"BUILD_BUILDID", false},
		{"BUILD_BUILDNUMBER", false},
		{"BUILD_BUILDURI", false},
		{"BUILD_REASON", false},
		{"AGENT_VERSION", false},
		{"TF_BUILD", false},
	}
}

func (r *AzurePipeline) RunURI() (url string) {
	teamFoundationServerURI := os.Getenv("SYSTEM_TEAMFOUNDATIONSERVERURI")
	definitionName := os.Getenv("SYSTEM_TEAMPROJECT")
	buildID := os.Getenv("BUILD_BUILDID")
	jobID := os.Getenv("SYSTEM_JOBID")

	uri, err := neturl.Parse(teamFoundationServerURI)
	if err != nil {
		return ""
	}

	query := neturl.Values{}
	query.Set("buildId", buildID)
	query.Set("view", "logs")
	query.Set("j", jobID)

	uri.Path = path.Join(uri.Path, definitionName, "_build/results")
	uri.RawQuery = query.Encode()

	return uri.String()
}

func (r *AzurePipeline) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}
