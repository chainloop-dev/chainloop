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

	attachments, err := pgv.requiredPolicyGroupsForMaterial(ctx, material)
	if err != nil {
		return nil, NewPolicyError(err)
	}

	for _, attachment := range attachments {
		// Load material content
		subject, err := getMaterialContent(material, path)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		ev, err := pgv.evaluatePolicyAttachment(ctx, attachment, subject,
			&evalOpts{kind: material.MaterialType, name: material.GetArtifact().GetId()},
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
		group, _, err := pgv.loadPolicyGroup(ctx, groupAtt)
		if err != nil {
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

func (pgv *PolicyGroupVerifier) requiredPolicyGroupsForMaterial(ctx context.Context, material *api.Attestation_Material) ([]*v1.PolicyAttachment, error) {
	result := make([]*v1.PolicyAttachment, 0)
	attachments := pgv.schema.GetPolicyGroups()

	for _, attachment := range attachments {
		// 1. load the policy group
		group, _, err := pgv.loadPolicyGroup(ctx, attachment)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		// 2. go through all policies in the group and check individually
		for _, policyAtt := range group.GetSpec().GetPolicies().GetMaterials() {
			apply, err := pgv.shouldApplyPolicy(ctx, policyAtt, material)
			if err != nil {
				return nil, err
			}

			if apply {
				result = append(result, policyAtt)
			}
		}
	}

	return result, nil
}

// LoadPolicySpec loads and validates a policy spec from a contract
func (pgv *PolicyGroupVerifier) loadPolicyGroup(ctx context.Context, attachment *v1.PolicyGroupAttachment) (*v1.PolicyGroup, *api.ResourceDescriptor, error) {
	loader, err := pgv.getGroupLoader(attachment)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get a loader for policy group: %w", err)
	}

	group, ref, err := loader.Load(ctx, attachment)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load policy group: %w", err)
	}

	// Validate just in case
	if err = validateResource(group); err != nil {
		return nil, nil, err
	}

	return group, ref, nil
}

func (pgv *PolicyGroupVerifier) getGroupLoader(attachment *v1.PolicyGroupAttachment) (GroupLoader, error) {
	ref := attachment.GetRef()

	if ref == "" {
		return nil, errors.New("policy group must be referenced in the attachment")
	}

	var loader GroupLoader
	scheme, _ := refParts(ref)
	switch scheme {
	// No scheme means chainloop loader
	case chainloopScheme, "":
		loader = NewChainloopGroupLoader(pgv.client)
	case fileScheme:
		loader = new(FileGroupLoader)
	case httpsScheme, httpScheme:
		loader = new(HTTPSGroupLoader)
	default:
		return nil, fmt.Errorf("policy scheme not supported: %q", scheme)
	}

	pgv.logger.Debug().Msgf("loading policy group %q using %T", ref, loader)

	return loader, nil
}
