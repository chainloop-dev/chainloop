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
	"context"
	"fmt"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	v12 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine/rego"
	"github.com/sigstore/cosign/v2/pkg/blob"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v2"
)

type PolicyVerifier struct {
	state *v12.CraftingState
	cas   *casclient.Client
}

func NewPolicyVerifier(state *v12.CraftingState, client *casclient.Client) *PolicyVerifier {
	// only Rego engine is currently supported
	return &PolicyVerifier{state: state, cas: client}
}

// Verify verifies that the statement is compliant with the policies present in the schema
func (pv *PolicyVerifier) Verify(ctx context.Context) ([]*engine.PolicyViolation, error) {
	violations := make([]*engine.PolicyViolation, 0)
	policies := pv.state.GetInputSchema().GetPolicies()
	for _, policyAtt := range policies {
		if policyAtt.Disabled {
			// policy is disabled
			// TODO: WARN.
			continue
		}
		spec, err := pv.loadSpec(policyAtt)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy spec: %w", err)
		}
		script, err := pv.loadPolicyScriptFromSpec(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy content: %w", err)
		}
		material, err := pv.loadSubject(policyAtt, spec, pv.state)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy subject: %w", err)
		}
		// verify policy, passing arguments from policyAtt
		ng := getPolicyEngine(spec)
		res, err := ng.Verify(ctx, script, material)
		if err != nil {
			return nil, fmt.Errorf("failed to verify policy: %w", err)
		}
		violations = append(violations, res...)
	}

	return violations, nil
}

func (pv *PolicyVerifier) loadSpec(attachment *v1.PolicyAttachment) (*v1.Policy, error) {
	// 1. look for the referenced policy spec (note: `name` is not supported yet)
	reference := attachment.GetRef()
	// this method understands env, http and https schemes, and defaults to file system.
	specContent, err := blob.LoadFileOrURL(reference)
	if err != nil {
		return nil, fmt.Errorf("loading policy spec: %w", err)
	}
	var policy v1.Policy
	if err := yaml.Unmarshal(specContent, &policy); err != nil {
		return nil, fmt.Errorf("unmarshalling policy spec: %w", err)
	}
	return &policy, nil
}

// loads a policy referenced from the spec
func (pv *PolicyVerifier) loadPolicyScriptFromSpec(spec *v1.Policy) (*engine.Policy, error) {
	var content []byte
	var err error
	if spec.GetSpec().GetEmbedded() != "" {
		content = []byte(spec.GetSpec().GetEmbedded())
	} else if spec.GetSpec().GetPath() != "" {
		content, err = blob.LoadFileOrURL(spec.GetSpec().GetPath())
		if err != nil {
			return nil, fmt.Errorf("loading policy content: %w", err)
		}
	} else {
		return nil, fmt.Errorf("policy spec is empty")
	}

	return &engine.Policy{
		Name:   spec.GetMetadata().GetName(),
		Source: content,
	}, nil
}

// load the subject of the policy.
func (pv *PolicyVerifier) loadSubject(attachment *v1.PolicyAttachment, spec *v1.Policy, state *v12.CraftingState) ([]byte, error) {
	// Load the affected material or attestation, and checks if the expected name and type match
	name := attachment.GetSelector().GetName()
	// if name selector is not set, the subject will become the full crafting state
	if name == "" {
		return protojson.Marshal(state.GetAttestation())
	}

	// if name is set, we want a specific material
	for _, m := range state.GetAttestation().GetMaterials() {
		if m.GetArtifact().GetName() == name {
			if spec.GetSpec().GetKind() != v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED && spec.GetSpec().GetKind() != m.GetMaterialType() {
				// If policy wasn't meant to be evaluated against this type of material, raise an error
				return nil, fmt.Errorf("invalid material type: %s, policy spected: %s", m.GetMaterialType(), spec.GetSpec().GetKind())
			}
			return pv.getMaterialPayload(m)
		}
	}

	return nil, fmt.Errorf("no material found with name %s", name)
}

// Gets the material payload from the CAS
func (pv *PolicyVerifier) getMaterialPayload(m *v12.Attestation_Material) ([]byte, error) {
	if !m.UploadedToCas {
		return m.GetArtifact().GetContent(), nil
	}
	// Look for material, and get its payload depending on which its nature.
	// switch m.MaterialType {
	// case v1.CraftingSchema_Mate
	// }

	return nil, nil
}

// getPolicyEngine returns a PolicyEngine implementation to evaluate a given policy.
func getPolicyEngine(_ *v1.Policy) engine.PolicyEngine {
	// Currently, only Rego is supported
	return new(rego.Rego)
}
