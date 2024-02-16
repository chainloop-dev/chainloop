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

type DaggerPipeline struct{}

func NewDaggerPipeline() *DaggerPipeline {
	return &DaggerPipeline{}
}

func (r *DaggerPipeline) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_DAGGER_PIPELINE
}

func (r *DaggerPipeline) CheckEnv() bool {
	for _, envVarName := range []string{"CHAINLOOP_DAGGER_CLIENT"} {
		if os.Getenv(envVarName) == "" {
			return false
		}
	}

	return true
}

func (r *DaggerPipeline) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{
		// Version of the Chainloop Client
		{"CHAINLOOP_DAGGER_CLIENT", false},
	}
}

// TODO: figure out an URL and or more useful information
func (r *DaggerPipeline) RunURI() string {
	return ""
}

func (r *DaggerPipeline) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}
