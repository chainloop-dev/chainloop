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
)

type AzurePipeline struct{}

const AzurePipelineID = "azure-pipeline"

func NewAzurePipeline() *AzurePipeline {
	return &AzurePipeline{}
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

func (r *AzurePipeline) ListEnvVars() []string {
	return []string{
		"BUILD_REQUESTEDFOREMAIL",
		"BUILD_REQUESTEDFOR",
		"BUILD_REPOSITORY_URI",
		"BUILD_REPOSITORY_NAME",
		"BUILD_BUILDID",
		"BUILD_BUILDNUMBER",
		"BUILD_BUILDURI",
		"BUILD_REASON",
		"AGENT_VERSION",
		"TF_BUILD",
	}
}

func (r *AzurePipeline) ResolveEnvVars() map[string]string {
	return resolveEnvVars(r.ListEnvVars())
}

func (r *AzurePipeline) String() string {
	return AzurePipelineID
}

func (r *AzurePipeline) RunURI() (url string) {
	teamFoundationServerURI := os.Getenv("SYSTEM_TEAMFOUNDATIONSERVERURI")
	definitionName := os.Getenv("SYSTEM_DEFINITIONNAME")
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
