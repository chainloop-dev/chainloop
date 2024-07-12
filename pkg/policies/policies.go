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
	"bufio"
	"bytes"
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	v12 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/chainloop-dev/chainloop/internal/grpcconn"

	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine/rego"
)

type PolicyVerifier struct {
	state   *v12.CraftingState
	casOpts *CASConnecitonOpts
	logger  *zerolog.Logger
}

type CASConnecitonOpts struct {
	CasAPI, CasCA string
	Insecure      bool
	CpConn        *grpc.ClientConn
}

func NewPolicyVerifier(state *v12.CraftingState, opts *CASConnecitonOpts, logger *zerolog.Logger) *PolicyVerifier {
	// only Rego engine is currently supported
	return &PolicyVerifier{state: state, casOpts: opts, logger: logger}
}

// Verify verifies that the statement is compliant with the policies present in the schema
func (pv *PolicyVerifier) Verify(ctx context.Context) ([]*engine.PolicyViolation, error) {
	violations := make([]*engine.PolicyViolation, 0)
	policies := pv.state.GetInputSchema().GetPolicies()
	for _, policyAtt := range policies {
		if policyAtt.Disabled {
			// policy is disabled
			pv.logger.Warn().Msgf("policy [name: %s, ref: %s] disabled", policyAtt.GetName(), policyAtt.GetRef())
			continue
		}

		// 1. load the policy spec
		spec, err := crafter.LoadPolicySpec(policyAtt)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy spec: %w", err)
		}

		// 2. load the policy script (rego)
		script, err := crafter.LoadPolicyScriptFromSpec(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy content: %w", err)
		}

		// 3. load the affected material (or the whole attestation)
		material, err := pv.loadSubject(ctx, policyAtt, spec)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy subject: %w", err)
		}

		pv.logger.Debug().Msgf("evaluating policy %s", spec.Metadata.Name)

		// 4. verify the policy
		ng := getPolicyEngine(spec)
		res, err := ng.Verify(ctx, script, material)
		if err != nil {
			return nil, fmt.Errorf("failed to verify policy: %w", err)
		}
		violations = append(violations, res...)

		// 5. Store result in the attestation itself (for the renderer to include them in the predicate)
		pv.state.Attestation.Policies = append(pv.state.Attestation.Policies, &v12.Policy{
			Name:       spec.Metadata.Name,
			Attachment: policyAtt,
			Violations: policyViolationsToAttestationViolations(res),
		})
	}

	return violations, nil
}

// load the subject of the policy.
func (pv *PolicyVerifier) loadSubject(ctx context.Context, attachment *v1.PolicyAttachment, spec *v1.Policy) ([]byte, error) {
	state := pv.state

	// Load the affected material or attestation, and checks if the expected name and type match
	name := attachment.GetSelector().GetName()
	// if name selector is not set, the subject will become the full crafting state
	if name == "" {
		return protojson.Marshal(state.GetAttestation())
	}

	// if name is set, we want a specific material
	for k, m := range state.GetAttestation().GetMaterials() {
		if k == name {
			if spec.GetSpec().GetKind() != v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED && spec.GetSpec().GetKind() != m.GetMaterialType() {
				// If policy wasn't meant to be evaluated against this type of material, raise an error
				return nil, fmt.Errorf("invalid material type: %s, policy expected: %s", m.GetMaterialType(), spec.GetSpec().GetKind())
			}
			return pv.getMaterialPayload(ctx, m)
		}
	}

	return nil, fmt.Errorf("no material found with name %s", name)
}

// Gets the material payload from the CAS
func (pv *PolicyVerifier) getMaterialPayload(ctx context.Context, m *v12.Attestation_Material) ([]byte, error) {
	if m.InlineCas {
		return m.GetArtifact().GetContent(), nil
	}

	// Use the CAS to look for the material
	var b bytes.Buffer
	w := bufio.NewWriter(&b)

	client, err := pv.getCASClient(pb.CASCredentialsServiceGetRequest_ROLE_DOWNLOADER, m.GetArtifact().GetDigest())
	if err != nil {
		return nil, err
	}
	err = client.Download(ctx, w, m.GetArtifact().GetDigest())
	if err != nil {
		return nil, fmt.Errorf("failed to download artifact: %w", err)
	}
	err = w.Flush()
	if err != nil {
		return nil, fmt.Errorf("failed to download artifact: %w", err)
	}

	return b.Bytes(), nil
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

// We need to create a connection for every single artifact, because it depends on the digest
func (pv *PolicyVerifier) getCASClient(role pb.CASCredentialsServiceGetRequest_Role, digest string) (*casclient.Client, error) {
	// Retrieve temporary credentials for uploading
	client := pb.NewCASCredentialsServiceClient(pv.casOpts.CpConn)
	resp, err := client.Get(context.Background(), &pb.CASCredentialsServiceGetRequest{
		Role:   role,
		Digest: digest,
	})
	if err != nil {
		return nil, err
	}

	if pv.casOpts.Insecure {
		pv.logger.Warn().Msg("API contacted in insecure mode")
	}

	var opts = []grpcconn.Option{
		grpcconn.WithInsecure(pv.casOpts.Insecure),
	}

	if pv.casOpts.CasCA != "" {
		opts = append(opts, grpcconn.WithCAFile(pv.casOpts.CasCA))
	}

	conn, err := grpcconn.New(pv.casOpts.CasAPI, resp.Result.Token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cas client: %w", err)
	}

	return casclient.New(conn, casclient.WithLogger(*pv.logger)), nil
}
