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

package biz

import (
	"context"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/pkg/policies"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	crafterAPI "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
)

type PolicyEvaluationUseCase struct {
	orgRepo OrganizationRepo
	logger  *log.Helper
}

func NewPolicyEvaluationUseCase(
	orgRepo OrganizationRepo,
	logger log.Logger,
) *PolicyEvaluationUseCase {
	return &PolicyEvaluationUseCase{
		orgRepo: orgRepo,
		logger:  log.NewHelper(logger),
	}
}

// Evaluate executes a generic policy evaluation without material context and returns the result
func (uc *PolicyEvaluationUseCase) Evaluate(ctx context.Context, opts *PolicyEvaluationEvaluateOpts) (*crafterAPI.PolicyEvaluation, error) {
	// Get current organization
	currentOrg := entities.CurrentOrg(ctx)
	if currentOrg == nil {
		return nil, NewErrNotFound("organization")
	}

	// Load full organization details to get policies_allowed_hostnames
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid organization ID: %w", err)
	}

	org, err := uc.orgRepo.FindByID(ctx, orgUUID)
	if err != nil {
		return nil, err
	}

	// Create policy attachment for evaluation
	ref := opts.PolicyReference
	scheme, _ := policies.RefParts(opts.PolicyReference)
	if scheme == "" {
		// If no scheme, assume it's a file path and add file:// prefix
		ref = fmt.Sprintf("file://%s", opts.PolicyReference)
	}

	attachment := &schemaapi.PolicyAttachment{
		Policy: &schemaapi.PolicyAttachment_Ref{Ref: ref},
		With:   opts.Inputs,
	}

	// Create policy verifier with organization's allowed hostnames
	zlog := zerolog.Ctx(ctx)
	if zlog == nil || zlog.GetLevel() == zerolog.Disabled {
		l := zerolog.New(zerolog.NewConsoleWriter())
		zlog = &l
	}

	verifierOpts := []policies.PolicyVerifierOption{
		policies.WithIncludeRawData(false),
	}
	if len(org.PoliciesAllowedHostnames) > 0 {
		verifierOpts = append(verifierOpts, policies.WithAllowedHostnames(org.PoliciesAllowedHostnames...))
	}

	pol := &schemaapi.Policies{}
	verifier := policies.NewPolicyVerifier(pol, nil, zlog, verifierOpts...)

	// Evaluate the policy
	policyEv, err := verifier.EvaluateGeneric(ctx, attachment)
	if err != nil {
		return nil, fmt.Errorf("evaluating policy: %w", err)
	}

	if policyEv == nil {
		return nil, fmt.Errorf("no execution branch matched, or all of them were ignored")
	}

	// Check if violations should block
	if len(policyEv.Violations) > 0 && org.BlockOnPolicyViolation {
		return policyEv, fmt.Errorf("policy evaluation failed with %d violation(s)", len(policyEv.Violations))
	}

	return policyEv, nil
}

type PolicyEvaluationEvaluateOpts struct {
	PolicyReference string
	Inputs          map[string]string
}
