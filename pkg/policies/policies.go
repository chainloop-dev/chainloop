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

package policies

import (
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine/rego"
)

// GetPolicyEngine returns a PolicyEngine implementation to evaluate a given policy.
func GetPolicyEngine(_ *v1.Policy) engine.PolicyEngine {
	// Currently, only Rego is supported
	return new(rego.Rego)
}
