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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bufbuild/protovalidate-go"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"github.com/sigstore/cosign/v2/pkg/blob"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	v12 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine/rego"
)

type PolicyVerifier struct {
	schema *v1.CraftingSchema
	logger *zerolog.Logger
}

func NewPolicyVerifier(schema *v1.CraftingSchema, logger *zerolog.Logger) *PolicyVerifier {
	// only Rego engine is currently supported
	return &PolicyVerifier{schema: schema, logger: logger}
}

// VerifyMaterial applies all required policies to a material
func (pv *PolicyVerifier) VerifyMaterial(ctx context.Context, material *v12.Attestation_Material, artifactPath string) ([]*v12.Policy, error) {
	result := make([]*v12.Policy, 0)
	policies, err := pv.requiredPoliciesForMaterial(material)
	if err != nil {
		return nil, fmt.Errorf("error getting required policies for material: %w", err)
	}
	for _, policy := range policies {
		// 1. load the policy spec
		spec, err := LoadPolicySpec(policy)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy spec: %w", err)
		}

		// load the policy script (rego)
		script, err := LoadPolicyScriptFromSpec(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy content: %w", err)
		}

		// Load material content
		subject, err := getMaterialContent(material, artifactPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load material content: %w", err)
		}

		pv.logger.Debug().Msgf("evaluating policy %s", spec.Metadata.Name)

		// verify the policy
		ng := getPolicyEngine(spec)
		res, err := ng.Verify(ctx, script, subject)
		if err != nil {
			return nil, fmt.Errorf("failed to verify policy: %w", err)
		}

		result = append(result, &v12.Policy{
			Name:         spec.GetMetadata().GetName(),
			MaterialName: material.GetArtifact().GetId(),
			Body:         string(script.Source),
			Violations:   engineViolationsToApiViolations(res),
		})
	}

	return result, nil
}

// VerifyStatement verifies that the statement is compliant with the policies present in the schema
func (pv *PolicyVerifier) VerifyStatement(ctx context.Context, statement *intoto.Statement) ([]*v12.Policy, error) {
	result := make([]*v12.Policy, 0)
	policies := pv.schema.GetPolicies().GetAttestation()
	for _, policyAtt := range policies {
		// 1. load the policy spec
		spec, err := LoadPolicySpec(policyAtt)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy spec: %w", err)
		}

		// it's expected statements can only be validated by policy of type ATTESTATION
		if spec.GetSpec().GetType() != v1.CraftingSchema_Material_ATTESTATION {
			continue
		}

		// 2. load the policy script (rego)
		script, err := LoadPolicyScriptFromSpec(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy content: %w", err)
		}

		pv.logger.Debug().Msgf("evaluating policy %s", spec.Metadata.Name)

		material, err := protojson.Marshal(statement)
		if err != nil {
			return nil, fmt.Errorf("failed to load material content: %w", err)
		}

		// 4. verify the policy
		ng := getPolicyEngine(spec)
		res, err := ng.Verify(ctx, script, material)
		if err != nil {
			return nil, fmt.Errorf("failed to verify policy: %w", err)
		}

		// 5. Store result in the attestation itself (for the renderer to include them in the predicate)
		result = append(result, &v12.Policy{
			Name:       spec.Metadata.Name,
			Body:       string(script.Source),
			Violations: policyViolationsToAttestationViolations(res),
		})
	}

	return result, nil
}

func engineViolationsToApiViolations(input []*engine.PolicyViolation) []*v12.Policy_Violation {
	res := make([]*v12.Policy_Violation, 0)
	for _, v := range input {
		res = append(res, &v12.Policy_Violation{
			Subject: v.Subject,
			Message: v.Violation,
		})
	}

	return res
}

func getMaterialContent(material *v12.Attestation_Material, artifactPath string) ([]byte, error) {
	if material.InlineCas {
		return material.GetArtifact().GetContent(), nil
	}

	if artifactPath == "" {
		return nil, errors.New("artifact path required")
	}

	// read content from local filesystem
	return os.ReadFile(artifactPath)
}

func (pv *PolicyVerifier) requiredPoliciesForMaterial(material *v12.Attestation_Material) ([]*v1.PolicyAttachment, error) {
	result := make([]*v1.PolicyAttachment, 0)
	policies := pv.schema.GetPolicies().GetMaterials()

	for _, policyAtt := range policies {
		// load the policy spec
		spec, err := LoadPolicySpec(policyAtt)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy spec: %w", err)
		}

		specType := spec.GetSpec().GetType()
		materialType := material.GetMaterialType()
		filteredName := policyAtt.GetSelector().GetName()

		// if spec has a type, and it's different to the material type, skip
		if specType != v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED && specType != materialType {
			// types don't match, continue
			continue
		}

		if filteredName != "" && filteredName != material.GetArtifact().GetId() {
			// a filer exists and doesn't match
			continue
		}

		// no type nor name to match, we can't guess anything
		if specType == v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED && filteredName == "" {
			continue
		}

		result = append(result, policyAtt)
	}

	return result, nil
}

// getPolicyEngine returns a PolicyEngine implementation to evaluate a given policy.
func getPolicyEngine(_ *v1.Policy) engine.PolicyEngine {
	// Currently, only Rego is supported
	return new(rego.Rego)
}

func policyViolationsToAttestationViolations(violations []*engine.PolicyViolation) (pvs []*v12.Policy_Violation) {
	for _, violation := range violations {
		pvs = append(pvs, &v12.Policy_Violation{
			Subject: violation.Subject,
			Message: violation.Violation,
		})
	}
	return
}

// LoadPolicySpec loads and validates a policy spec from a contract
func LoadPolicySpec(attachment *v1.PolicyAttachment) (*v1.Policy, error) {
	if attachment.GetEmbedded() != nil {
		return attachment.GetEmbedded(), nil
	}

	// if policy is not embedded in the contract, we'll look for it

	// look for the referenced policy spec (note: loading by `name` is not supported yet)
	reference := attachment.GetRef()
	// this method understands env, http and https schemes, and defaults to file system.
	rawData, err := blob.LoadFileOrURL(reference)
	if err != nil {
		return nil, fmt.Errorf("loading policy spec: %w", err)
	}
	jsonContent, err := materials.LoadJSONBytes(rawData, filepath.Ext(reference))
	if err != nil {
		return nil, fmt.Errorf("loading policy spec: %w", err)
	}
	var policy v1.Policy
	if err := protojson.Unmarshal(jsonContent, &policy); err != nil {
		return nil, fmt.Errorf("unmarshalling policy spec: %w", err)
	}
	// Validate just in case
	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("validating policy spec: %w", err)
	}
	err = validator.Validate(&policy)
	if err != nil {
		return nil, fmt.Errorf("validating policy spec: %w", err)
	}

	return &policy, nil
}

// LoadPolicyScriptFromSpec loads a policy referenced from the spec
func LoadPolicyScriptFromSpec(spec *v1.Policy) (*engine.Policy, error) {
	var content []byte
	var err error

	switch source := spec.GetSpec().GetSource().(type) {
	case *v1.PolicySpec_Embedded:
		content = []byte(source.Embedded)
	case *v1.PolicySpec_Path:
		content, err = blob.LoadFileOrURL(source.Path)
		if err != nil {
			return nil, fmt.Errorf("loading policy content: %w", err)
		}
	default:
		return nil, fmt.Errorf("policy spec is empty")
	}

	return &engine.Policy{
		Name:   spec.GetMetadata().GetName(),
		Source: content,
	}, nil
}
