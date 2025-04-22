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
	"context"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

type Generic struct{}

func NewGeneric() *Generic {
	return &Generic{}
}

func (r *Generic) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_RUNNER_TYPE_UNSPECIFIED
}

func (r *Generic) CheckEnv() bool {
	return true
}

// Returns a list of environment variables names. This list is used to
// automatically inject environment variables into the attestation.
func (r *Generic) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{}
}

func (r *Generic) RunURI() string {
	return ""
}

func (r *Generic) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}

func (r *Generic) WorkflowFilePath(_ context.Context) string {
	return ""
}

func (r *Generic) IsHosted(_ context.Context) bool {
	return false
}
