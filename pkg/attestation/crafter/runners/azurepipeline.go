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

package runners

import (
	"context"
	"net/url"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
)

// Azure Pipelines predefined variables used to build the run link.
const (
	azureCollectionURIEnv = "SYSTEM_TEAMFOUNDATIONCOLLECTIONURI"
	azureServerURIEnv     = "SYSTEM_TEAMFOUNDATIONSERVERURI"
	azureTeamProjectEnv   = "SYSTEM_TEAMPROJECT"
)

type AzurePipeline struct {
	*Generic
}

func NewAzurePipeline() *AzurePipeline {
	return &AzurePipeline{
		NewGeneric(),
	}
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
		{"BUILD_REQUESTEDFOREMAIL", true},
		{"BUILD_REQUESTEDFOR", false},
		{"BUILD_REPOSITORY_URI", true},
		{"BUILD_REPOSITORY_NAME", true},
		{"BUILD_BUILDID", false},
		{"BUILD_BUILDNUMBER", false},
		{"BUILD_BUILDURI", false},
		{"BUILD_REASON", true},
		{"TF_BUILD", true},
	}
}

// RunURI returns a browser link to the pipeline run, in the form
// {collectionURI}{project}/_build/results?buildId={buildID}. It returns an
// empty string if the required variables are missing.
func (r *AzurePipeline) RunURI() string {
	// The collection URI includes the collection segment on Azure DevOps Server
	// (https://host/tfs/DefaultCollection/), which the server URI does not have.
	// On Azure DevOps Services both point to https://dev.azure.com/{org}/.
	baseURI := os.Getenv(azureCollectionURIEnv)
	if baseURI == "" {
		baseURI = os.Getenv(azureServerURIEnv)
	}

	project := os.Getenv(azureTeamProjectEnv)
	buildID := os.Getenv("BUILD_BUILDID")
	if baseURI == "" || project == "" || buildID == "" {
		return ""
	}

	uri, err := url.Parse(baseURI)
	if err != nil || uri.Scheme == "" || uri.Host == "" {
		return ""
	}

	query := url.Values{}
	query.Set("buildId", buildID)
	// Link to the logs of this job when its ID is known.
	if jobID := os.Getenv("SYSTEM_JOBID"); jobID != "" {
		query.Set("view", "logs")
		query.Set("j", jobID)
	}

	// JoinPath expects escaped elements and drops the path if one does not unescape.
	uri = uri.JoinPath(url.PathEscape(project), "_build", "results")
	uri.RawQuery = query.Encode()

	return uri.String()
}

func (r *AzurePipeline) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}

func (r *AzurePipeline) WorkflowFilePath() string {
	return ""
}

func (r *AzurePipeline) IsAuthenticated() bool {
	return false
}

func (r *AzurePipeline) Environment() RunnerEnvironment {
	return Unknown
}

func (r *AzurePipeline) VerifyCommitSignature(_ context.Context, _ string) *commitverification.CommitVerification {
	return nil // Not supported for this runner
}

func (r *AzurePipeline) Report(_ []byte, _ string) error {
	return nil
}
