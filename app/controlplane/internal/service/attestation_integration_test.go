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
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/attjwtmiddleware"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

// Regression tests for PFM-6717: a project-scoped API token must not be able to retrieve
// another project's workflow metadata and contract schema through AttestationService/GetContract.
// Attestation endpoints are skipped by the authz middleware, so the handler is the only
// authorization point.
type getContractRBACIntegrationSuite struct {
	testhelpers.UseCasesEachTestSuite
	org                  *biz.Organization
	projectA, projectB   *biz.Project
	workflowA, workflowB *biz.Workflow
	projectToken         *biz.APIToken
	workflowToken        *biz.APIToken
	orgToken             *biz.APIToken
	productToken         *biz.APIToken
	emptyProductToken    *biz.APIToken
	productID            uuid.UUID
	emptyProductID       uuid.UUID
	svc                  *AttestationService
}

func (s *getContractRBACIntegrationSuite) SetupTest() {
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	ctx := context.Background()
	var err error

	s.org, err = s.Organization.Create(ctx, "get-contract-rbac-org")
	s.Require().NoError(err)

	s.projectA, err = s.Project.Create(ctx, s.org.ID, "project-a")
	s.Require().NoError(err)
	s.projectB, err = s.Project.Create(ctx, s.org.ID, "project-b")
	s.Require().NoError(err)

	s.workflowA, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "workflow-a", OrgID: s.org.ID, Project: "project-a"})
	s.Require().NoError(err)
	s.workflowB, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "workflow-b", OrgID: s.org.ID, Project: "project-b"})
	s.Require().NoError(err)

	// A token confined to project A, one pinned to workflow A, and an org-wide token.
	s.projectToken, err = s.APIToken.Create(ctx, "token-project-a", nil, nil, &s.org.ID, biz.APITokenWithProject(s.projectA))
	s.Require().NoError(err)
	s.workflowToken, err = s.APIToken.Create(ctx, "token-workflow-a", nil, nil, &s.org.ID, biz.APITokenWithProject(s.projectA), biz.APITokenWithWorkflow(s.workflowA))
	s.Require().NoError(err)
	s.orgToken, err = s.APIToken.Create(ctx, "token-org", nil, nil, &s.org.ID)
	s.Require().NoError(err)

	// A token confined to a product holding project A only, and one whose product holds
	// nothing — the shape a token has once its product is deleted.
	s.productID, s.emptyProductID = uuid.New(), uuid.New()
	s.productToken, err = s.APIToken.Create(ctx, "token-product", nil, nil, &s.org.ID,
		biz.APITokenWithScope(authz.ResourceTypeProduct, s.productID, "checkout-platform"))
	s.Require().NoError(err)
	s.emptyProductToken, err = s.APIToken.Create(ctx, "token-empty-product", nil, nil, &s.org.ID,
		biz.APITokenWithScope(authz.ResourceTypeProduct, s.emptyProductID, "empty-product"))
	s.Require().NoError(err)

	orgUUID := uuid.MustParse(s.org.ID)
	s.Require().NoError(s.Repos.Membership.AddResourceRole(ctx, orgUUID,
		authz.ResourceTypeProject, s.projectA.ID, authz.MembershipTypeAPIToken,
		s.productToken.ID, authz.RoleProjectAdmin, nil))

	authzUC := biz.NewAuthzUseCase(&biz.AuthzUseCaseConfig{
		CasbinEnforcer: s.Enforcer,
		APITokenRepo:   s.Repos.APITokenRepo,
		Logger:         s.L,
	})

	s.svc = NewAttestationService(&NewAttestationServiceOpts{
		WorkflowRunUC:      s.WorkflowRun,
		WorkflowUC:         s.Workflow,
		WorkflowContractUC: s.WorkflowContract,
		OrgUC:              s.Organization,
		ProjectUC:          s.Project,
		ProjectVersionUC:   s.ProjectVersion,
		Opts:               []NewOpt{WithProjectUseCase(s.Project), WithEnforcer(authzUC)},
	})
}

func (s *getContractRBACIntegrationSuite) TestProjectScopedToken() {
	s.Run("cannot read another project's workflow and contract", func() {
		_, err := s.svc.GetContract(s.ctxForToken(s.projectToken), &pb.AttestationServiceGetContractRequest{
			ProjectName:  s.projectB.Name,
			WorkflowName: s.workflowB.Name,
		})
		s.Require().Error(err)
		s.True(kerrors.IsForbidden(err), "expected forbidden, got %v", err)
	})

	s.Run("can read its own project's workflow and contract", func() {
		resp, err := s.svc.GetContract(s.ctxForToken(s.projectToken), &pb.AttestationServiceGetContractRequest{
			ProjectName:  s.projectA.Name,
			WorkflowName: s.workflowA.Name,
		})
		s.Require().NoError(err)
		s.Equal(s.workflowA.Name, resp.GetResult().GetWorkflow().GetName())
		s.NotNil(resp.GetResult().GetContract())
		s.Equal(s.projectA.Name, resp.GetResult().GetWorkflow().GetProject())
	})
}

func (s *getContractRBACIntegrationSuite) TestWorkflowScopedToken() {
	// Workflow-scoped tokens are already confined by findWorkflowFromTokenOrNameOrRunID;
	// this guards against regressions on top of the project-level check.
	s.Run("cannot read another project's workflow and contract", func() {
		_, err := s.svc.GetContract(s.ctxForToken(s.workflowToken), &pb.AttestationServiceGetContractRequest{
			ProjectName:  s.projectB.Name,
			WorkflowName: s.workflowB.Name,
		})
		s.Require().Error(err)
		s.True(kerrors.IsForbidden(err), "expected forbidden, got %v", err)
	})

	s.Run("can read its own workflow and contract", func() {
		resp, err := s.svc.GetContract(s.ctxForToken(s.workflowToken), &pb.AttestationServiceGetContractRequest{
			ProjectName:  s.projectA.Name,
			WorkflowName: s.workflowA.Name,
		})
		s.Require().NoError(err)
		s.Equal(s.workflowA.Name, resp.GetResult().GetWorkflow().GetName())
		s.NotNil(resp.GetResult().GetContract())
	})
}

func (s *getContractRBACIntegrationSuite) TestOrgScopedToken() {
	// Backward compatibility: org-wide tokens keep reaching every workflow in the organization.
	for _, wf := range []*biz.Workflow{s.workflowA, s.workflowB} {
		resp, err := s.svc.GetContract(s.ctxForToken(s.orgToken), &pb.AttestationServiceGetContractRequest{
			ProjectName:  wf.Project,
			WorkflowName: wf.Name,
		})
		s.Require().NoError(err)
		s.Equal(wf.Name, resp.GetResult().GetWorkflow().GetName())
		s.NotNil(resp.GetResult().GetContract())
	}

	// The workflow lookup itself requires an explicit project name, for every caller.
	_, err := s.svc.GetContract(s.ctxForToken(s.orgToken), &pb.AttestationServiceGetContractRequest{
		WorkflowName: s.workflowA.Name,
	})
	s.Require().Error(err)
	s.True(kerrors.IsBadRequest(err), "expected bad request, got %v", err)
}

// ctxForToken builds a context equivalent to the one the attestation middlewares produce for
// an API-token authenticated call: current org, current API token, the api-token authz
// subject and the robot account the attestation handlers read the organization from.
func (s *getContractRBACIntegrationSuite) ctxForToken(token *biz.APIToken) context.Context {
	ctx := entities.WithCurrentOrg(context.Background(), &entities.Org{ID: s.org.ID, Name: s.org.Name})
	ctx = entities.WithCurrentAPIToken(ctx, &entities.APIToken{
		ID:           token.ID.String(),
		Name:         token.Name,
		ProjectID:    token.ProjectID,
		ProjectName:  token.ProjectName,
		WorkflowID:   token.WorkflowID,
		WorkflowName: token.WorkflowName,
		Scope:        token.Scope,
		ScopeID:      token.ScopeID,
		ScopeName:    token.ScopeName,
	})
	ctx = usercontext.WithAuthzSubject(ctx, (&authz.SubjectAPIToken{ID: token.ID.String()}).String())

	return usercontext.WithRobotAccount(ctx, &usercontext.RobotAccount{
		OrgID:       s.org.ID,
		ProviderKey: attjwtmiddleware.APITokenProviderKey,
	})
}

// ctxForScopedToken adds what WithCurrentMembershipsMiddleware loads for a scoped token: the
// memberships that decide which projects it reaches.
func (s *getContractRBACIntegrationSuite) ctxForScopedToken(token *biz.APIToken) context.Context {
	ctx := s.ctxForToken(token)

	mm, err := s.Membership.ListAllMembershipsForAPIToken(context.Background(), token.ID)
	s.Require().NoError(err)

	resources := make([]*entities.ResourceMembership, 0, len(mm))
	for _, m := range mm {
		resources = append(resources, &entities.ResourceMembership{
			MembershipID: m.ID, Role: m.Role, ResourceType: m.ResourceType, ResourceID: m.ResourceID,
		})
	}

	return entities.WithMembership(ctx, &entities.Membership{
		MemberID: token.ID, MemberType: authz.MembershipTypeAPIToken, Resources: resources,
	})
}

// A product-scoped token reaches exactly the projects its product currently holds.
func (s *getContractRBACIntegrationSuite) TestProductScopedToken() {
	s.Run("can read the workflow of a project its product holds", func() {
		resp, err := s.svc.GetContract(s.ctxForScopedToken(s.productToken), &pb.AttestationServiceGetContractRequest{
			ProjectName:  s.projectA.Name,
			WorkflowName: s.workflowA.Name,
		})
		s.Require().NoError(err)
		s.Equal(s.workflowA.Name, resp.GetResult().GetWorkflow().GetName())
		s.NotNil(resp.GetResult().GetContract())
	})

	s.Run("cannot read a project its product does not hold", func() {
		_, err := s.svc.GetContract(s.ctxForScopedToken(s.productToken), &pb.AttestationServiceGetContractRequest{
			ProjectName:  s.projectB.Name,
			WorkflowName: s.workflowB.Name,
		})
		s.Require().Error(err)
		s.True(kerrors.IsForbidden(err), "expected forbidden, got %v", err)
		// The refusal must name the product and the resource, and must not be an internal
		// error from dereferencing a project name the token does not have.
		s.Contains(err.Error(), "checkout-platform")
		s.Contains(err.Error(), "project")
	})
}

// A token whose product holds no projects authorizes nothing — the shape it has once the
// product is deleted. It must not widen to the whole organization.
func (s *getContractRBACIntegrationSuite) TestProductScopedTokenWithNoMemberships() {
	for _, wf := range []*biz.Workflow{s.workflowA, s.workflowB} {
		_, err := s.svc.GetContract(s.ctxForScopedToken(s.emptyProductToken), &pb.AttestationServiceGetContractRequest{
			ProjectName:  wf.Project,
			WorkflowName: wf.Name,
		})
		s.Require().Error(err, "an empty scope denies everything")
		s.True(kerrors.IsForbidden(err), "expected forbidden, got %v", err)
		s.Contains(err.Error(), "empty-product")
	}
}

func TestGetContractRBACIntegration(t *testing.T) {
	suite.Run(t, new(getContractRBACIntegrationSuite))
}
