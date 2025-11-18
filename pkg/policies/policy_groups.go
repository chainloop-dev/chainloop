//
// Copyright 2024-2025 The Chainloop Authors.
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

	v13 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/templates"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
)

type PolicyGroupVerifier struct {
	policyGroups []*v1.PolicyGroupAttachment
	logger       *zerolog.Logger
	client       v13.AttestationServiceClient

	*PolicyVerifier
}

var _ Verifier = (*PolicyGroupVerifier)(nil)

func NewPolicyGroupVerifier(policyGroups []*v1.PolicyGroupAttachment, policies *v1.Policies, client v13.AttestationServiceClient, logger *zerolog.Logger, opts ...PolicyVerifierOption) *PolicyGroupVerifier {
	return &PolicyGroupVerifier{policyGroups: policyGroups, client: client, logger: logger,
		PolicyVerifier: NewPolicyVerifier(policies, client, logger, opts...)}
}

// VerifyMaterial evaluates a material against groups of policies defined in the schema
func (pgv *PolicyGroupVerifier) VerifyMaterial(ctx context.Context, material *api.Attestation_Material, path string) ([]*api.PolicyEvaluation, error) {
	result := make([]*api.PolicyEvaluation, 0)

	groupAtts := pgv.policyGroups

	for _, groupAtt := range groupAtts {
		// 1. load the policy group
		group, desc, err := LoadPolicyGroup(ctx, groupAtt, &LoadPolicyGroupOptions{
			Client: pgv.client,
			Logger: pgv.logger,
		})
		if err != nil {
			return nil, NewPolicyError(err)
		}

		// matches group arguments against spec and apply defaults
		groupArgs, err := ComputeArguments(group.GetMetadata().GetName(), group.GetSpec().GetInputs(), groupAtt.GetWith(), nil, pgv.logger)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		// Validate skip list and log warnings for unknown policy names
		if err := pgv.validateSkipList(ctx, group, groupAtt); err != nil {
			pgv.logger.Warn().Err(err).Msg("some policies in skip list were not found in the policy group")
		}

		// gather required policies
		policyAtts, err := pgv.requiredPoliciesForMaterial(ctx, material, group, groupAtt, groupArgs)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		for _, policyAtt := range policyAtts {
			// Load material content
			subject, err := material.GetEvaluableContent(path)
			if err != nil {
				return nil, NewPolicyError(err)
			}

			ev, err := pgv.evaluatePolicyAttachment(ctx, policyAtt, subject,
				&evalOpts{kind: material.MaterialType, name: material.GetId(), bindings: groupArgs},
			)
			if err != nil {
				return nil, NewPolicyError(err)
			}

			if ev == nil {
				// no evaluation, skip
				continue
			}

			// Assign group reference to this evaluation
			ev.GroupReference = &api.PolicyEvaluation_Reference{
				Name:    group.GetMetadata().GetName(),
				Digest:  desc.GetDigest(),
				Uri:     desc.GetURI(),
				OrgName: desc.GetOrgName(),
			}
			result = append(result, ev)
		}
	}

	return result, nil
}

func (pgv *PolicyGroupVerifier) VerifyStatement(ctx context.Context, statement *intoto.Statement) ([]*api.PolicyEvaluation, error) {
	result := make([]*api.PolicyEvaluation, 0)
	attachments := pgv.policyGroups
	for _, groupAtt := range attachments {
		group, desc, err := LoadPolicyGroup(ctx, groupAtt, &LoadPolicyGroupOptions{
			Client: pgv.client,
			Logger: pgv.logger,
		})
		if err != nil {
			// Temporarily skip if policy groups still use old schema
			// TODO: remove this check in next release
			pgv.logger.Warn().Msgf("policy group '%s' skipped since it's not found or it might use an old schema version", groupAtt.GetRef())
			continue
		}
		// compute group arguments
		groupArgs, err := ComputeArguments(group.GetMetadata().GetName(), group.GetSpec().GetInputs(), groupAtt.GetWith(), nil, pgv.logger)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		// Validate skip list and log warnings for unknown policy names
		if err := pgv.validateSkipList(ctx, group, groupAtt); err != nil {
			pgv.logger.Warn().Err(err).Msg("some policies in skip list were not found in the policy group")
		}

		for _, attachment := range group.GetSpec().GetPolicies().GetAttestation() {
			// Check if policy should be skipped
			policyName, err := pgv.getPolicyName(ctx, attachment)
			if err != nil {
				return nil, NewPolicyError(fmt.Errorf("failed to get policy name: %w", err))
			}

			// Skip if policy name is in the skip list
			if slices.Contains(groupAtt.GetSkip(), policyName) {
				pgv.logger.Debug().Str("policy", policyName).Msg("skipping attestation policy per skip list")
				continue
			}

			material, err := protojson.Marshal(statement)
			if err != nil {
				return nil, NewPolicyError(err)
			}

			ev, err := pgv.evaluatePolicyAttachment(ctx, attachment, material,
				&evalOpts{kind: v1.CraftingSchema_Material_ATTESTATION, bindings: groupArgs},
			)
			if err != nil {
				return nil, NewPolicyError(err)
			}

			if ev == nil {
				// no evaluation, skip
				continue
			}

			// Assign group reference to this evaluation
			ev.GroupReference = &api.PolicyEvaluation_Reference{
				Name:    group.GetMetadata().GetName(),
				Digest:  desc.GetDigest(),
				Uri:     desc.GetURI(),
				OrgName: desc.GetOrgName(),
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
	scheme, _ := RefParts(ref)
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

// Gets the policies that can be applied to a material within a group
func (pgv *PolicyGroupVerifier) requiredPoliciesForMaterial(ctx context.Context, material *api.Attestation_Material, group *v1.PolicyGroup, groupAtt *v1.PolicyGroupAttachment, groupArgs map[string]string) ([]*v1.PolicyAttachment, error) {
	result := make([]*v1.PolicyAttachment, 0)

	// 2. go through all materials in the group and look for the crafted material
	for _, groupMaterial := range group.GetSpec().GetPolicies().GetMaterials() {
		gm, err := InterpolateGroupMaterial(groupMaterial, groupArgs)
		if err != nil {
			return nil, err
		}

		if gm.Name != "" && gm.Name != material.GetId() {
			continue
		}

		// 3. Material found or group material has no name. Let's check policies to apply
		// Note that this looks for types supported by the policies, not by the group material (it's ignored in that case)
		for _, policyAtt := range gm.GetPolicies() {
			apply, err := pgv.shouldApplyPolicy(ctx, policyAtt, material)
			if err != nil {
				return nil, err
			}

			if apply {
				// Check if policy should be skipped
				policyName, err := pgv.getPolicyName(ctx, policyAtt)
				if err != nil {
					return nil, fmt.Errorf("failed to get policy name: %w", err)
				}

				// Skip if policy name is in the skip list
				if slices.Contains(groupAtt.GetSkip(), policyName) {
					pgv.logger.Debug().Str("policy", policyName).Msg("skipping policy per skip list")
					continue
				}

				result = append(result, policyAtt)
			}
		}
	}

	return result, nil
}

// InterpolateGroupMaterial returns a version of the group material with all template interpolations applied (only name is supported atm)
func InterpolateGroupMaterial(gm *v1.PolicyGroup_Material, bindings map[string]string) (*v1.PolicyGroup_Material, error) {
	name := gm.Name
	name, err := templates.ApplyBinding(name, bindings)
	if err != nil {
		return nil, err
	}

	return &v1.PolicyGroup_Material{
		Type:     gm.Type,
		Name:     name,
		Optional: gm.Optional,
		Policies: gm.Policies,
	}, nil
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
		// types match
		return true, nil
	}

	// if policy doesn't have any type to match, we can apply it
	if len(specTypes) == 0 {
		return true, nil
	}

	return false, nil
}

// getPolicyName extracts the metadata.name from a PolicyAttachment
// It handles both embedded and referenced policies by loading the policy spec when needed
func (pgv *PolicyGroupVerifier) getPolicyName(ctx context.Context, attachment *v1.PolicyAttachment) (string, error) {
	// Case 1: Embedded policy - direct access
	if embedded := attachment.GetEmbedded(); embedded != nil {
		return embedded.GetMetadata().GetName(), nil
	}

	// Case 2: Referenced policy - must load it
	if ref := attachment.GetRef(); ref != "" {
		// Load the policy spec using existing loader infrastructure
		policy, _, err := pgv.loadPolicySpec(ctx, attachment)
		if err != nil {
			return "", fmt.Errorf("failed to load policy to get name: %w", err)
		}
		return policy.GetMetadata().GetName(), nil
	}

	// Should never happen due to protobuf validation, but handle defensively
	return "", errors.New("policy attachment has neither ref nor embedded policy")
}

// validateSkipList checks if policy names in the skip list exist in the group
// and returns an error if any unknown policy names are found
func (pgv *PolicyGroupVerifier) validateSkipList(ctx context.Context, group *v1.PolicyGroup, groupAtt *v1.PolicyGroupAttachment) error {
	if len(groupAtt.GetSkip()) == 0 {
		return nil
	}

	// Collect all policy names in the group
	policyNames := make(map[string]bool)

	// Collect material policy names
	for _, groupMaterial := range group.GetSpec().GetPolicies().GetMaterials() {
		for _, policyAtt := range groupMaterial.GetPolicies() {
			name, err := pgv.getPolicyName(ctx, policyAtt)
			if err != nil {
				pgv.logger.Warn().Err(err).Msg("failed to get policy name during skip list validation")
				continue
			}
			policyNames[name] = true
		}
	}

	// Collect attestation policy names
	for _, policyAtt := range group.GetSpec().GetPolicies().GetAttestation() {
		name, err := pgv.getPolicyName(ctx, policyAtt)
		if err != nil {
			pgv.logger.Warn().Err(err).Msg("failed to get policy name during skip list validation")
			continue
		}
		policyNames[name] = true
	}

	// Check each skip entry against collected policy names and collect unknown ones
	var unknownPolicies []string
	for _, skipName := range groupAtt.GetSkip() {
		if !policyNames[skipName] {
			unknownPolicies = append(unknownPolicies, skipName)
		}
	}

	// Return error if there are unknown policies
	if len(unknownPolicies) > 0 {
		return fmt.Errorf("policies in skip list not found in group %q: %v", group.GetMetadata().GetName(), unknownPolicies)
	}

	return nil
}
