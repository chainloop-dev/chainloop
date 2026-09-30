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
	"time"

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

	// A token confined to a product holding project A only, and one whose product holds no
	// projects at all — the shape a token has once its product is deleted.
	s.productID, s.emptyProductID = uuid.New(), uuid.New()
	s.productToken, err = s.APIToken.Create(ctx, "token-product", nil, nil, &s.org.ID,
		biz.APITokenWithScope(authz.ResourceTypeProduct, &s.productID),
		biz.APITokenWithProjectIDs([]uuid.UUID{s.projectA.ID}))
	s.Require().NoError(err)
	s.emptyProductToken, err = s.APIToken.Create(ctx, "token-empty-product", nil, nil, &s.org.ID,
		biz.APITokenWithScope(authz.ResourceTypeProduct, &s.emptyProductID),
		biz.APITokenWithProjectIDs(nil))
	s.Require().NoError(err)

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
		ProjectIDs:   token.ProjectIDs,
	})
	ctx = usercontext.WithAuthzSubject(ctx, (&authz.SubjectAPIToken{ID: token.ID.String()}).String())

	return usercontext.WithRobotAccount(ctx, &usercontext.RobotAccount{
		OrgID:       s.org.ID,
		ProviderKey: attjwtmiddleware.APITokenProviderKey,
	})
}

// A product-scoped token reaches exactly the projects its product currently holds.
func (s *getContractRBACIntegrationSuite) TestProductScopedToken() {
	s.Run("can read the workflow of a project its product holds", func() {
		resp, err := s.svc.GetContract(s.ctxForToken(s.productToken), &pb.AttestationServiceGetContractRequest{
			ProjectName:  s.projectA.Name,
			WorkflowName: s.workflowA.Name,
		})
		s.Require().NoError(err)
		s.Equal(s.workflowA.Name, resp.GetResult().GetWorkflow().GetName())
		s.NotNil(resp.GetResult().GetContract())
	})

	s.Run("cannot read a project its product does not hold", func() {
		_, err := s.svc.GetContract(s.ctxForToken(s.productToken), &pb.AttestationServiceGetContractRequest{
			ProjectName:  s.projectB.Name,
			WorkflowName: s.workflowB.Name,
		})
		s.Require().Error(err)
		s.True(kerrors.IsForbidden(err), "expected forbidden, got %v", err)
		// The refusal must name the product and the resource, and must not be an internal
		// error from dereferencing a project name the token does not have.
		s.Contains(err.Error(), s.productID.String())
		s.Contains(err.Error(), "project")
	})
}

// A token whose product holds no projects authorizes nothing — the shape it has once the
// product is deleted. It must not widen to the whole organization.
func (s *getContractRBACIntegrationSuite) TestProductScopedTokenWithNoProjects() {
	for _, wf := range []*biz.Workflow{s.workflowA, s.workflowB} {
		_, err := s.svc.GetContract(s.ctxForToken(s.emptyProductToken), &pb.AttestationServiceGetContractRequest{
			ProjectName:  wf.Project,
			WorkflowName: wf.Name,
		})
		s.Require().Error(err, "an empty scope denies everything")
		s.True(kerrors.IsForbidden(err), "expected forbidden, got %v", err)
		s.Contains(err.Error(), s.emptyProductID.String())
	}
}

// A project soft-deleted after it was written into a token's project list must not stay
// reachable. The id stays on the list until the platform next syncs it, and is inert meanwhile:
// every project lookup filters out a deleted project, so the contract lookup a product token makes
// finds nothing, as it would for any other caller. The list itself is exercised by the unlisted
// and empty-list cases above.
func (s *getContractRBACIntegrationSuite) TestProductScopedTokenStaleProjectIDIsInert() {
	ctx := context.Background()
	s.Require().NoError(s.Data.DB.Project.UpdateOneID(s.projectA.ID).SetDeletedAt(time.Now()).Exec(ctx))

	_, err := s.svc.GetContract(s.ctxForToken(s.productToken), &pb.AttestationServiceGetContractRequest{
		ProjectName:  s.projectA.Name,
		WorkflowName: s.workflowA.Name,
	})
	s.Require().Error(err, "a soft-deleted project must not still be reachable through a stale id")
	// The id is inert because every project lookup filters out a deleted project (the workflow
	// lookup 404s before the RBAC/ReachesProject check ever runs) — not because the token's own
	// project list was pruned.
	s.True(kerrors.IsNotFound(err), "expected not found, got %v", err)
}

func TestGetContractRBACIntegration(t *testing.T) {
	suite.Run(t, new(getContractRBACIntegrationSuite))
}

// Regression tests for the legacy robot-account credential: a robot account is scoped to a single
// workflow, but findWorkflowFromTokenOrNameOrRunID resolved the workflow purely from the request,
// so any robot account could reach every workflow in its organization. Robot accounts can no longer
// be issued (CP-N4) and there is no revoke path left in the product, so existing tokens must be
// confined by the handler.
//
// usercontext.RobotAccount is a carrier shared with the API-token and federated middlewares, which
// leave WorkflowID empty; only legacy robot accounts carry one, and those callers must be untouched.
type robotAccountWorkflowBindingIntegrationSuite struct {
	testhelpers.UseCasesEachTestSuite
	org                  *biz.Organization
	projectA, projectB   *biz.Project
	workflowA, workflowB *biz.Workflow
	orgToken             *biz.APIToken
	svc                  *AttestationService
}

func (s *robotAccountWorkflowBindingIntegrationSuite) SetupTest() {
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	ctx := context.Background()
	var err error

	s.org, err = s.Organization.Create(ctx, "robot-account-binding-org")
	s.Require().NoError(err)

	s.projectA, err = s.Project.Create(ctx, s.org.ID, "project-a")
	s.Require().NoError(err)
	s.projectB, err = s.Project.Create(ctx, s.org.ID, "project-b")
	s.Require().NoError(err)

	s.workflowA, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "workflow-a", OrgID: s.org.ID, Project: "project-a"})
	s.Require().NoError(err)
	s.workflowB, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "workflow-b", OrgID: s.org.ID, Project: "project-b"})
	s.Require().NoError(err)

	s.orgToken, err = s.APIToken.Create(ctx, "token-org", nil, nil, &s.org.ID)
	s.Require().NoError(err)

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

func (s *robotAccountWorkflowBindingIntegrationSuite) TestGetContractWorkflowBinding() {
	testCases := []struct {
		name string
		// ctx is built per-case because the credential shape is what is under test.
		ctx func() context.Context
		// target workflow of the request
		wantWorkflow  *biz.Workflow
		wantForbidden bool
	}{
		{
			name:         "legacy robot account reaches the workflow it is bound to",
			ctx:          func() context.Context { return s.ctxForRobotAccount(s.workflowA) },
			wantWorkflow: s.workflowA,
		},
		{
			name:          "legacy robot account cannot reach another workflow in the same org",
			ctx:           func() context.Context { return s.ctxForRobotAccount(s.workflowA) },
			wantWorkflow:  s.workflowB,
			wantForbidden: true,
		},
		{
			name:         "org-scoped API token is unaffected by the binding",
			ctx:          func() context.Context { return s.ctxForOrgAPIToken() },
			wantWorkflow: s.workflowB,
		},
		{
			name:         "federated credential is unaffected by the binding",
			ctx:          func() context.Context { return s.ctxForFederated() },
			wantWorkflow: s.workflowB,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			resp, err := s.svc.GetContract(tc.ctx(), &pb.AttestationServiceGetContractRequest{
				ProjectName:  tc.wantWorkflow.Project,
				WorkflowName: tc.wantWorkflow.Name,
			})

			if tc.wantForbidden {
				s.Require().Error(err)
				s.True(kerrors.IsForbidden(err), "expected forbidden, got %v", err)
				return
			}

			s.Require().NoError(err)
			s.Equal(tc.wantWorkflow.Name, resp.GetResult().GetWorkflow().GetName())
			s.NotNil(resp.GetResult().GetContract())
		})
	}
}

// ctxForRobotAccount mirrors the context WithAttestationContextFromRobotAccount produces for a
// legacy robot-account token: the account is pinned to exactly one workflow.
func (s *robotAccountWorkflowBindingIntegrationSuite) ctxForRobotAccount(wf *biz.Workflow) context.Context {
	ctx := entities.WithCurrentOrg(context.Background(), &entities.Org{ID: s.org.ID, Name: s.org.Name})

	return usercontext.WithRobotAccount(ctx, &usercontext.RobotAccount{
		ID:          uuid.NewString(),
		WorkflowID:  wf.ID.String(),
		OrgID:       s.org.ID,
		ProviderKey: attjwtmiddleware.RobotAccountProviderKey,
	})
}

// ctxForOrgAPIToken mirrors the API-token middleware: it reuses the RobotAccount carrier but
// leaves WorkflowID empty.
func (s *robotAccountWorkflowBindingIntegrationSuite) ctxForOrgAPIToken() context.Context {
	ctx := entities.WithCurrentOrg(context.Background(), &entities.Org{ID: s.org.ID, Name: s.org.Name})
	ctx = entities.WithCurrentAPIToken(ctx, &entities.APIToken{
		ID:      s.orgToken.ID.String(),
		Name:    s.orgToken.Name,
		Scope:   s.orgToken.Scope,
		ScopeID: s.orgToken.ScopeID,
	})
	ctx = usercontext.WithAuthzSubject(ctx, (&authz.SubjectAPIToken{ID: s.orgToken.ID.String()}).String())

	return usercontext.WithRobotAccount(ctx, &usercontext.RobotAccount{
		OrgID:       s.org.ID,
		ProviderKey: attjwtmiddleware.APITokenProviderKey,
	})
}

// ctxForFederated mirrors the federated middleware, which also leaves WorkflowID empty.
func (s *robotAccountWorkflowBindingIntegrationSuite) ctxForFederated() context.Context {
	ctx := entities.WithCurrentOrg(context.Background(), &entities.Org{ID: s.org.ID, Name: s.org.Name})

	return usercontext.WithRobotAccount(ctx, &usercontext.RobotAccount{
		OrgID:       s.org.ID,
		ProviderKey: attjwtmiddleware.FederatedProviderKey,
	})
}

func TestRobotAccountWorkflowBindingIntegration(t *testing.T) {
	suite.Run(t, new(robotAccountWorkflowBindingIntegrationSuite))
}
