//
// Copyright 2025 The Chainloop Authors.
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

package action

import (
	"context"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/internal/policydevel"
	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	attestationapi "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/policies"
)

type PolicyEvaluateOpts struct {
	MaterialPath string
	Kind         string
	Annotations  map[string]string
	PolicyPath   string
	Inputs       map[string]string
}

type PolicyEvaluate struct {
	*ActionsOpts
	opts *PolicyEvaluateOpts
}

func NewPolicyEvaluate(opts *PolicyEvaluateOpts, actionOpts *ActionsOpts) (*PolicyEvaluate, error) {
	if actionOpts.CPConnection == nil {
		return nil, fmt.Errorf("control plane connection is required")
	}

	return &PolicyEvaluate{
		ActionsOpts: actionOpts,
		opts:        opts,
	}, nil
}

func (action *PolicyEvaluate) Run(ctx context.Context) (*attestationapi.PolicyEvaluation, error) {
	// 1. Get organization settings
	contextClient := pb.NewContextServiceClient(action.CPConnection)
	contextResp, err := contextClient.Current(ctx, &pb.ContextServiceCurrentRequest{})
	if err != nil {
		return nil, fmt.Errorf("fetching organization settings: %w", err)
	}

	if contextResp.Result == nil || contextResp.Result.CurrentMembership == nil || contextResp.Result.CurrentMembership.Org == nil {
		return nil, fmt.Errorf("no organization context found")
	}

	org := contextResp.Result.CurrentMembership.Org
	allowedHostnames := org.PolicyAllowedHostnames

	// 2. Create policy attachment
	ref := action.opts.PolicyPath
	scheme, _ := policies.RefParts(action.opts.PolicyPath)
	if scheme == "" {
		// If no scheme, assume it's a file path and add file:// prefix
		ref = fmt.Sprintf("file://%s", action.opts.PolicyPath)
	}

	policyAttachment := &schemaapi.PolicyAttachment{
		Policy: &schemaapi.PolicyAttachment_Ref{Ref: ref},
		With:   action.opts.Inputs,
	}

	// 3. Create policies structure based on whether we have a material
	var pol *schemaapi.Policies
	if action.opts.MaterialPath != "" {
		// Material-based evaluation
		pol = &schemaapi.Policies{
			Materials: []*schemaapi.PolicyAttachment{policyAttachment},
		}
	} else {
		// Generic evaluation
		pol = &schemaapi.Policies{}
	}

	// 4. Create policy verifier with organization's allowed hostnames
	verifierOpts := []policies.PolicyVerifierOption{
		policies.WithIncludeRawData(false),
		policies.WithEnablePrint(false),
		policies.WithGRPCConn(action.CPConnection),
	}
	if len(allowedHostnames) > 0 {
		verifierOpts = append(verifierOpts, policies.WithAllowedHostnames(allowedHostnames...))
	}

	attClient := pb.NewAttestationServiceClient(action.CPConnection)
	verifier := policies.NewPolicyVerifier(pol, attClient, &action.Logger, verifierOpts...)

	// 5. Evaluate: either material-based or generic
	if action.opts.MaterialPath != "" {
		// Material-based evaluation
		material, err := policydevel.CraftMaterial(action.opts.MaterialPath, action.opts.Kind, &action.Logger)
		if err != nil {
			return nil, fmt.Errorf("crafting material: %w", err)
		}
		material.Annotations = action.opts.Annotations

		policyEvs, err := verifier.VerifyMaterial(ctx, material, action.opts.MaterialPath)
		if err != nil {
			return nil, fmt.Errorf("evaluating policy against material: %w", err)
		}

		if len(policyEvs) == 0 || policyEvs[0] == nil {
			return nil, fmt.Errorf("no execution branch matched, or all of them were ignored, for kind %s", material.MaterialType.String())
		}

		return policyEvs[0], nil
	}

	// Generic evaluation
	policyEv, err := verifier.EvaluateGeneric(ctx, policyAttachment)
	if err != nil {
		return nil, fmt.Errorf("evaluating policy: %w", err)
	}

	if policyEv == nil {
		return nil, fmt.Errorf("no execution branch matched, or all of them were ignored")
	}

	return policyEv, nil
}
