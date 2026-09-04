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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/oci"
	creds "github.com/chainloop-dev/chainloop/pkg/credentials/mocks"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

const (
	digestProjectA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestProjectB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	digestOrgWide  = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

// Regression tests for PFM-6716: a project-scoped API token must not be able to resolve a CAS
// mapping outside its project through any of the two download endpoints, GetDownloadURL and
// CASCredentialsService.Get, which share the same mapping lookup.
type casDownloadRBACIntegrationSuite struct {
	testhelpers.UseCasesEachTestSuite
	org                *biz.Organization
	projectA, projectB *biz.Project
	backend            *biz.CASBackend
	projectToken       *biz.APIToken
	orgToken           *biz.APIToken
	redirectSvc        *CASRedirectService
	credentialsSvc     *CASCredentialsService
}

func (s *casDownloadRBACIntegrationSuite) SetupTest() {
	credsWriter := creds.NewReaderWriter(s.T())
	credsWriter.On("SaveCredentials", mock.Anything, mock.Anything, mock.Anything).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	ctx := context.Background()
	var err error

	s.org, err = s.Organization.Create(ctx, "cas-download-rbac-org")
	s.Require().NoError(err)

	s.projectA, err = s.Project.Create(ctx, s.org.ID, "project-a")
	s.Require().NoError(err)
	s.projectB, err = s.Project.Create(ctx, s.org.ID, "project-b")
	s.Require().NoError(err)

	s.backend, err = s.CASBackend.Create(ctx, s.org.ID, "rbac-backend", "my-location", "backend for RBAC tests", oci.ProviderID, nil, true, false, nil)
	s.Require().NoError(err)

	// A token confined to project A and an org-wide token, both carrying the default
	// artifact-download policy.
	s.projectToken, err = s.APIToken.Create(ctx, "token-project-a", nil, nil, &s.org.ID, biz.APITokenWithProject(s.projectA))
	s.Require().NoError(err)
	s.orgToken, err = s.APIToken.Create(ctx, "token-org", nil, nil, &s.org.ID)
	s.Require().NoError(err)

	// Artifacts scoped to each project plus one with no scope at all, reachable with
	// organization-wide access only.
	for digest, project := range map[string]*biz.Project{
		digestProjectA: s.projectA,
		digestProjectB: s.projectB,
	} {
		_, err = s.CASMapping.Create(ctx, digest, s.backend.ID.String(), &biz.CASMappingCreateOpts{ProjectID: &project.ID})
		s.Require().NoError(err)
	}

	_, err = s.CASMapping.Create(ctx, digestOrgWide, s.backend.ID.String(), nil)
	s.Require().NoError(err)

	// Services wired as in production, with a throwaway EC key for the CAS JWT builder.
	casUC, err := biz.NewCASCredentialsUseCase(&conf.Auth{CasRobotAccountPrivateKeyPath: s.writeECPrivateKey()})
	s.Require().NoError(err)

	authzUC := biz.NewAuthzUseCase(&biz.AuthzUseCaseConfig{
		CasbinEnforcer: s.Enforcer,
		APITokenRepo:   s.Repos.APITokenRepo,
		Logger:         s.L,
	})

	s.redirectSvc, err = NewCASRedirectService(s.CASMapping, casUC, &conf.Bootstrap_CASServer{DownloadUrl: "https://cas.example.test/download"})
	s.Require().NoError(err)

	s.credentialsSvc = NewCASCredentialsService(casUC, s.CASMapping, s.CASBackend, authzUC)
}

func (s *casDownloadRBACIntegrationSuite) TestGetDownloadURL() {
	s.Run("project-scoped token cannot download another project's artifact", func() {
		_, err := s.redirectSvc.GetDownloadURL(s.ctxForToken(s.projectToken), &pb.GetDownloadURLRequest{Digest: digestProjectB})
		s.Require().Error(err)
		s.True(kerrors.IsNotFound(err), "expected not found, got %v", err)
	})

	s.Run("project-scoped token cannot download an org-wide artifact", func() {
		_, err := s.redirectSvc.GetDownloadURL(s.ctxForToken(s.projectToken), &pb.GetDownloadURLRequest{Digest: digestOrgWide})
		s.Require().Error(err)
		s.True(kerrors.IsNotFound(err), "expected not found, got %v", err)
	})

	s.Run("project-scoped token can download its own project's artifact", func() {
		resp, err := s.redirectSvc.GetDownloadURL(s.ctxForToken(s.projectToken), &pb.GetDownloadURLRequest{Digest: digestProjectA})
		s.Require().NoError(err)
		s.Contains(resp.GetResult().GetUrl(), digestProjectA)
	})

	s.Run("org-scoped token still reaches the whole organization", func() {
		for _, digest := range []string{digestProjectA, digestProjectB, digestOrgWide} {
			resp, err := s.redirectSvc.GetDownloadURL(s.ctxForToken(s.orgToken), &pb.GetDownloadURLRequest{Digest: digest})
			s.Require().NoError(err)
			s.Contains(resp.GetResult().GetUrl(), digest)
		}
	})
}

func (s *casDownloadRBACIntegrationSuite) TestCASCredentialsGet() {
	s.Run("project-scoped token cannot get downloader credentials for another project's artifact", func() {
		_, err := s.credentialsSvc.Get(s.ctxForToken(s.projectToken), &pb.CASCredentialsServiceGetRequest{
			Digest: digestProjectB,
			Role:   pb.CASCredentialsServiceGetRequest_ROLE_DOWNLOADER,
		})
		s.Require().Error(err)
		s.True(kerrors.IsForbidden(err), "expected forbidden, got %v", err)
	})

	s.Run("project-scoped token cannot get downloader credentials for an org-wide artifact", func() {
		_, err := s.credentialsSvc.Get(s.ctxForToken(s.projectToken), &pb.CASCredentialsServiceGetRequest{
			Digest: digestOrgWide,
			Role:   pb.CASCredentialsServiceGetRequest_ROLE_DOWNLOADER,
		})
		s.Require().Error(err)
		s.True(kerrors.IsForbidden(err), "expected forbidden, got %v", err)
	})

	s.Run("project-scoped token can get downloader credentials for its own project's artifact", func() {
		resp, err := s.credentialsSvc.Get(s.ctxForToken(s.projectToken), &pb.CASCredentialsServiceGetRequest{
			Digest: digestProjectA,
			Role:   pb.CASCredentialsServiceGetRequest_ROLE_DOWNLOADER,
		})
		s.Require().NoError(err)
		s.NotEmpty(resp.GetResult().GetToken())
	})

	s.Run("org-scoped token still reaches the whole organization", func() {
		resp, err := s.credentialsSvc.Get(s.ctxForToken(s.orgToken), &pb.CASCredentialsServiceGetRequest{
			Digest: digestProjectB,
			Role:   pb.CASCredentialsServiceGetRequest_ROLE_DOWNLOADER,
		})
		s.Require().NoError(err)
		s.NotEmpty(resp.GetResult().GetToken())
	})
}

// ctxForToken builds a context equivalent to the one the authz middlewares produce for an
// API-token authenticated call: current org, current API token and the api-token authz subject.
func (s *casDownloadRBACIntegrationSuite) ctxForToken(token *biz.APIToken) context.Context {
	ctx := entities.WithCurrentOrg(context.Background(), &entities.Org{ID: s.org.ID, Name: s.org.Name})
	ctx = entities.WithCurrentAPIToken(ctx, &entities.APIToken{
		ID:        token.ID.String(),
		Name:      token.Name,
		ProjectID: token.ProjectID,
	})

	return usercontext.WithAuthzSubject(ctx, (&authz.SubjectAPIToken{ID: token.ID.String()}).String())
}

func (s *casDownloadRBACIntegrationSuite) writeECPrivateKey() string {
	// The CAS JWT builder signs with ES512, which requires a P-521 curve.
	key, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	s.Require().NoError(err)

	der, err := x509.MarshalECPrivateKey(key)
	s.Require().NoError(err)

	path := filepath.Join(s.T().TempDir(), "cas-key.pem")
	s.Require().NoError(os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600))

	return path
}

func TestCASDownloadRBACIntegration(t *testing.T) {
	suite.Run(t, new(casDownloadRBACIntegrationSuite))
}
