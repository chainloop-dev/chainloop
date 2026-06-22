//
// Copyright 2026 The Chainloop Authors.
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

package service

import (
	"context"
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/suite"
)

const (
	applyContractName = "svc-apply-contract"

	applyContractV1 = `
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: svc-apply-contract
spec:
  materials:
    - type: ARTIFACT
      name: my-artifact
`
	// Same contract with an extra material, so the raw body differs
	applyContractV2 = `
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: svc-apply-contract
spec:
  materials:
    - type: ARTIFACT
      name: my-artifact
    - type: SBOM_CYCLONEDX_JSON
      name: my-sbom
`
)

func (s *workflowContractApplyIntegrationTestSuite) apply(rawSchema string, dryRun bool) *pb.WorkflowContractServiceApplyResponse {
	resp, err := s.svc.Apply(s.ctx, &pb.WorkflowContractServiceApplyRequest{
		RawSchema: []byte(rawSchema),
		DryRun:    dryRun,
	})
	s.Require().NoError(err)
	return resp
}

func (s *workflowContractApplyIntegrationTestSuite) latestRevision() int {
	contract, err := s.WorkflowContract.FindByNameInOrg(s.ctx, s.org.ID, applyContractName)
	if err != nil && biz.IsNotFound(err) {
		return 0
	}
	s.Require().NoError(err)
	if contract == nil {
		return 0
	}
	return contract.LatestRevision
}

func (s *workflowContractApplyIntegrationTestSuite) TestApply() {
	// 1 - Real apply creates the contract
	resp := s.apply(applyContractV1, false)
	s.Equal(pb.WorkflowContractServiceApplyResponse_APPLY_STATUS_CREATED, resp.GetStatus())
	s.True(resp.GetChanged())
	s.EqualValues(1, resp.GetCurrentRevision())
	s.Equal(1, s.latestRevision())

	// 2 - Dry run with identical content reports unchanged and does not persist
	resp = s.apply(applyContractV1, true)
	s.Equal(pb.WorkflowContractServiceApplyResponse_APPLY_STATUS_UNCHANGED, resp.GetStatus())
	s.False(resp.GetChanged())
	s.EqualValues(1, resp.GetCurrentRevision())
	s.Equal(1, s.latestRevision())

	// 3 - Dry run with different content reports updated and does not persist
	resp = s.apply(applyContractV2, true)
	s.Equal(pb.WorkflowContractServiceApplyResponse_APPLY_STATUS_UPDATED, resp.GetStatus())
	s.True(resp.GetChanged())
	s.EqualValues(1, resp.GetCurrentRevision())
	s.Equal(1, s.latestRevision(), "dry run must not bump the revision")

	// 4 - Real apply with different content bumps the revision
	resp = s.apply(applyContractV2, false)
	s.Equal(pb.WorkflowContractServiceApplyResponse_APPLY_STATUS_UPDATED, resp.GetStatus())
	s.True(resp.GetChanged())
	s.EqualValues(2, resp.GetCurrentRevision())
	s.Equal(2, s.latestRevision())

	// 5 - Real apply with the same content again reports unchanged
	resp = s.apply(applyContractV2, false)
	s.Equal(pb.WorkflowContractServiceApplyResponse_APPLY_STATUS_UNCHANGED, resp.GetStatus())
	s.False(resp.GetChanged())
	s.EqualValues(2, resp.GetCurrentRevision())

	// 6 - Dry run for a brand new contract reports created with revision 0 and does not persist
	resp, err := s.svc.Apply(s.ctx, &pb.WorkflowContractServiceApplyRequest{
		RawSchema: []byte(`
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: svc-apply-contract-new
spec:
  materials:
    - type: ARTIFACT
      name: my-artifact
`),
		DryRun: true,
	})
	s.Require().NoError(err)
	s.Equal(pb.WorkflowContractServiceApplyResponse_APPLY_STATUS_CREATED, resp.GetStatus())
	s.True(resp.GetChanged())
	s.EqualValues(0, resp.GetCurrentRevision())

	created, err := s.WorkflowContract.FindByNameInOrg(s.ctx, s.org.ID, "svc-apply-contract-new")
	if err != nil {
		s.True(biz.IsNotFound(err), "dry run must not create the contract")
	} else {
		s.Nil(created, "dry run must not create the contract")
	}
}

// contractWithPolicyRef builds a v2 contract that references a policy via a provider scheme.
func contractWithPolicyRef(name, policyRef string) string {
	return `
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: ` + name + `
spec:
  materials:
    - type: ARTIFACT
      name: my-artifact
  policies:
    attestation:
      - ref: ` + policyRef + `
`
}

// contractWithPolicyGroupRef builds a v2 contract that references a policy group via a provider scheme.
func contractWithPolicyGroupRef(name, groupRef string) string {
	return `
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: ` + name + `
spec:
  materials:
    - type: ARTIFACT
      name: my-artifact
  policyGroups:
    - ref: ` + groupRef + `
`
}

// TestApplyBatchExemption verifies that references to resources declared as part of the same
// batch apply are treated as known (not resolved against the registry), while references to
// resources not in the batch are still validated - in dry-run as well as on a real apply.
// No policy provider is configured in this harness, so any non-exempt provider-scheme reference
// fails resolution, which is exactly what proves remote references are still validated.
func (s *workflowContractApplyIntegrationTestSuite) TestApplyBatchExemption() {
	testCases := []struct {
		name                  string
		rawSchema             string
		batchPolicyNames      []string
		batchPolicyGroupNames []string
		dryRun                bool
		wantErr               bool
	}{
		{
			name:             "batch-local policy exempted in dry-run",
			rawSchema:        contractWithPolicyRef("svc-apply-batch-pol-dry", "chainloop://batch-pol"),
			batchPolicyNames: []string{"batch-pol"},
			dryRun:           true,
		},
		{
			name:             "batch-local policy exempted on real apply",
			rawSchema:        contractWithPolicyRef("svc-apply-batch-pol-real", "chainloop://batch-pol"),
			batchPolicyNames: []string{"batch-pol"},
			dryRun:           false,
		},
		{
			name:                  "batch-local policy group exempted in dry-run",
			rawSchema:             contractWithPolicyGroupRef("svc-apply-batch-grp-dry", "chainloop://batch-grp"),
			batchPolicyGroupNames: []string{"batch-grp"},
			dryRun:                true,
		},
		{
			name:      "non-batch policy still validated in dry-run",
			rawSchema: contractWithPolicyRef("svc-apply-remote-pol-dry", "chainloop://not-in-batch"),
			dryRun:    true,
			wantErr:   true,
		},
		{
			name:      "non-batch policy group still validated in dry-run",
			rawSchema: contractWithPolicyGroupRef("svc-apply-remote-grp-dry", "chainloop://not-in-batch-grp"),
			dryRun:    true,
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			resp, err := s.svc.Apply(s.ctx, &pb.WorkflowContractServiceApplyRequest{
				RawSchema:             []byte(tc.rawSchema),
				DryRun:                tc.dryRun,
				BatchPolicyNames:      tc.batchPolicyNames,
				BatchPolicyGroupNames: tc.batchPolicyGroupNames,
			})

			if tc.wantErr {
				s.Require().Error(err)
				return
			}

			s.Require().NoError(err)
			s.Equal(pb.WorkflowContractServiceApplyResponse_APPLY_STATUS_CREATED, resp.GetStatus())
			s.True(resp.GetChanged())
		})
	}
}

func TestWorkflowContractApply(t *testing.T) {
	suite.Run(t, new(workflowContractApplyIntegrationTestSuite))
}

type workflowContractApplyIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org *biz.Organization
	svc *WorkflowContractService
	ctx context.Context
}

func (s *workflowContractApplyIntegrationTestSuite) SetupTest() {
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	var err error
	s.org, err = s.Organization.CreateWithRandomName(context.Background())
	s.Require().NoError(err)

	s.svc = NewWorkflowSchemaService(s.WorkflowContract, s.Organization, s.User)

	// Build a context with the current org and a bearer token, as the handler expects
	ctx := entities.WithCurrentOrg(context.Background(), &entities.Org{ID: s.org.ID, Name: s.org.Name})
	ctx = transport.NewServerContext(ctx, &applyMockTransport{
		header: applyMockHeader{"Authorization": "Bearer test-token"},
	})
	s.ctx = ctx
}

type applyMockHeader map[string]string

func (h applyMockHeader) Get(key string) string { return h[key] }
func (h applyMockHeader) Set(key, value string) { h[key] = value }
func (h applyMockHeader) Add(key, value string) { h[key] = value }
func (h applyMockHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}

func (h applyMockHeader) Values(key string) []string {
	if v, ok := h[key]; ok {
		return []string{v}
	}
	return nil
}

type applyMockTransport struct {
	header transport.Header
}

func (tr *applyMockTransport) Kind() transport.Kind            { return transport.KindGRPC }
func (tr *applyMockTransport) Endpoint() string                { return "" }
func (tr *applyMockTransport) Operation() string               { return "" }
func (tr *applyMockTransport) RequestHeader() transport.Header { return tr.header }
func (tr *applyMockTransport) ReplyHeader() transport.Header   { return tr.header }
