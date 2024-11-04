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
	"slices"

	"github.com/bufbuild/protovalidate-go"
	v13 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
)

type PolicyGroupVerifier struct {
	schema *v1.CraftingSchema
	logger *zerolog.Logger
	client v13.AttestationServiceClient

	*PolicyVerifier
}

var _ Verifier = (*PolicyGroupVerifier)(nil)

func NewPolicyGroupVerifier(schema *v1.CraftingSchema, client v13.AttestationServiceClient, logger *zerolog.Logger) *PolicyGroupVerifier {
	return &PolicyGroupVerifier{schema: schema, client: client, logger: logger,
		PolicyVerifier: NewPolicyVerifier(schema, client, logger)}
}

// VerifyMaterial evaluates a material against groups of policies defined in the schema
func (pgv *PolicyGroupVerifier) VerifyMaterial(ctx context.Context, material *api.Attestation_Material, path string) ([]*api.PolicyEvaluation, error) {
	result := make([]*api.PolicyEvaluation, 0)

	attachments, err := pgv.requiredPoliciesForMaterial(ctx, material)
	if err != nil {
		return nil, NewPolicyError(err)
	}

	for _, attachment := range attachments {
		// Load material content
		subject, err := material.GetEvaluableContent(path)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		ev, err := pgv.evaluatePolicyAttachment(ctx, attachment, subject,
			&evalOpts{kind: material.MaterialType, name: material.GetID()},
		)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		result = append(result, ev)
	}

	return result, nil
}

func (pgv *PolicyGroupVerifier) VerifyStatement(ctx context.Context, statement *intoto.Statement) ([]*api.PolicyEvaluation, error) {
	result := make([]*api.PolicyEvaluation, 0)
	attachments := pgv.schema.GetPolicyGroups()
	for _, groupAtt := range attachments {
		group, _, err := LoadPolicyGroup(ctx, groupAtt, &LoadPolicyGroupOptions{
			Client: pgv.client,
			Logger: pgv.logger,
		})
		if err != nil {
			// Temporarily skip if policy groups still use old schema
			// TODO: remove this check in next release
			var ve *protovalidate.ValidationError
			if errors.As(err, &ve) {
				pgv.logger.Warn().Msgf("policy group '%s' skipped since it uses an old schema version", groupAtt.GetRef())
				continue
			}
			return nil, NewPolicyError(err)
		}
		for _, attachment := range group.GetSpec().GetPolicies().GetAttestation() {
			material, err := protojson.Marshal(statement)
			if err != nil {
				return nil, NewPolicyError(err)
			}

			ev, err := pgv.evaluatePolicyAttachment(ctx, attachment, material,
				&evalOpts{kind: v1.CraftingSchema_Material_ATTESTATION},
			)
			if err != nil {
				return nil, NewPolicyError(err)
			}

			result = append(result, ev)
		}
	}

	return result, nil
}

type LoadPolicyGroupOptions struct {
	Client v13.AttestationServiceClient
	Logger *zerolog.Logger
}

// LoadPolicyGroup loads a group (unmarshalls it) from a group attachment
func LoadPolicyGroup(ctx context.Context, att *v1.PolicyGroupAttachment, opts *LoadPolicyGroupOptions) (*v1.PolicyGroup, *PolicyDescriptor, error) {
	loader, err := getGroupLoader(att, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get a loader for policy group: %w", err)
	}

	group, ref, err := loader.Load(ctx, att)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load policy group: %w", err)
	}

	// Validate just in case
	if err = validateResource(group); err != nil {
		return nil, nil, err
	}

	return group, ref, nil
}

// getGroupLoader creates a suitable group loader for a group attachment
func getGroupLoader(attachment *v1.PolicyGroupAttachment, opts *LoadPolicyGroupOptions) (GroupLoader, error) {
	ref := attachment.GetRef()

	if ref == "" {
		return nil, errors.New("policy group must be referenced in the attachment")
	}

	var loader GroupLoader
	scheme, _ := refParts(ref)
	switch scheme {
	// No scheme means chainloop loader
	case chainloopScheme, "":
		loader = NewChainloopGroupLoader(opts.Client)
	case fileScheme:
		loader = new(FileGroupLoader)
	case httpsScheme, httpScheme:
		loader = new(HTTPSGroupLoader)
	default:
		return nil, fmt.Errorf("policy scheme not supported: %q", scheme)
	}

	opts.Logger.Debug().Msgf("loading policy group %q using %T", ref, loader)

	return loader, nil
}

func (pgv *PolicyGroupVerifier) requiredPoliciesForMaterial(ctx context.Context, material *api.Attestation_Material) ([]*v1.PolicyAttachment, error) {
	result := make([]*v1.PolicyAttachment, 0)

	attachments := pgv.schema.GetPolicyGroups()

	for _, attachment := range attachments {
		// 1. load the policy group
		group, _, err := LoadPolicyGroup(ctx, attachment, &LoadPolicyGroupOptions{
			Client: pgv.client,
			Logger: pgv.logger,
		})
		if err != nil {
			// Temporarily skip if policy groups still use old schema
			// TODO: remove this check in next release
			var ve *protovalidate.ValidationError
			if errors.As(err, &ve) {
				pgv.logger.Warn().Msgf("policy group '%s' skipped since it uses an old schema version", attachment.GetRef())
				continue
			}
			return nil, NewPolicyError(err)
		}

		// 2. go through all materials in the group and look for the crafted material
		for _, schemaMaterial := range group.GetSpec().GetPolicies().GetMaterials() {
			if schemaMaterial.GetName() != material.GetID() {
				continue
			}

			// 3. Material found. Let's check its policies
			for _, policyAtt := range schemaMaterial.GetPolicies() {
				apply, err := pgv.shouldApplyPolicy(ctx, policyAtt, material)
				if err != nil {
					return nil, err
				}

				if apply {
					result = append(result, policyAtt)
				}
			}
		}
	}

	return result, nil
}

// // policy groups can be applied if they support the material type, or they don't have any specified material
func (pgv *PolicyGroupVerifier) shouldApplyPolicy(ctx context.Context, policyAtt *v1.PolicyAttachment, material *api.Attestation_Material) (bool, error) {
	// load the policy spec
	spec, _, err := pgv.loadPolicySpec(ctx, policyAtt)
	if err != nil {
		return false, fmt.Errorf("failed to load policy attachment %q: %w", policyAtt.GetRef(), err)
	}

	materialType := material.GetMaterialType()
	specTypes := getPolicyTypes(spec)

	// if spec has a type, and matches, it can be applied
	if len(specTypes) > 0 && slices.Contains(specTypes, materialType) {
		// types don't match, continue
		return true, nil
	}

	// if policy doesn't have any type to match, we can apply it
	if len(specTypes) == 0 {
		return true, nil
	}

	return false, nil
}
