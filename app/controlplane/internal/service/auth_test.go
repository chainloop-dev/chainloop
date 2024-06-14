//
// Copyright 2023 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestGetAuthURLs(t *testing.T) {
	internalServer := &conf.Server_HTTP{Addr: "1.2.3.4"}
	testCases := []struct {
		name    string
		config  *conf.Server_HTTP
		want    *AuthURLs
		wantErr bool
	}{
		{
			name:   "neither external url nor externalAddr set",
			config: internalServer,
			want:   &AuthURLs{callback: "http://1.2.3.4/auth/callback", Login: "http://1.2.3.4/auth/login"},
		},
		{
			name:   "correct URL, http",
			config: &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "http://foo.com"},
			want:   &AuthURLs{callback: "http://foo.com/auth/callback", Login: "http://foo.com/auth/login"},
		},
		{
			name:   "correct URL, https",
			config: &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "https://foo.com"},
			want:   &AuthURLs{callback: "https://foo.com/auth/callback", Login: "https://foo.com/auth/login"},
		},
		{
			name:   "with path",
			config: &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "https://foo.com/path"},
			want:   &AuthURLs{callback: "https://foo.com/path/auth/callback", Login: "https://foo.com/path/auth/login"},
		},
		{
			name:   "with port",
			config: &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "https://foo.com:1234"},
			want:   &AuthURLs{callback: "https://foo.com:1234/auth/callback", Login: "https://foo.com:1234/auth/login"},
		},
		{
			name:    "invalid, missing scheme",
			config:  &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "localhost.com"},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getAuthURLs(tc.config)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAuthOnboarding(t *testing.T) {
	suite.Run(t, new(authOnboardingTestSuite))
}

type authOnboardingTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	usr, usr1 *biz.User
	org       *biz.Organization
	m         *biz.Membership
}

func (s *authOnboardingTestSuite) SetupTest() {
	t := s.T()
	assert := assert.New(t)
	ctx := context.Background()

	s.TestingUseCases = testhelpers.NewTestingUseCases(t)

	s.setupUsersAndOrganization(ctx, assert)
	s.setupMembership(ctx, assert)
}

func (s *authOnboardingTestSuite) setupUsersAndOrganization(ctx context.Context, assert *assert.Assertions) {
	var err error
	s.usr, err = s.User.FindOrCreateByEmail(ctx, "foo@bar")
	assert.NoError(err)

	s.org, err = s.Organization.Create(ctx, "onboarded-org")
	assert.NoError(err)

	s.usr1, err = s.User.FindOrCreateByEmail(ctx, "bar@foo")
	assert.NoError(err)
}

func (s *authOnboardingTestSuite) setupMembership(ctx context.Context, assert *assert.Assertions) {
	var err error
	s.m, err = s.Membership.Create(ctx, s.org.ID, s.usr1.ID, biz.WithMembershipRole(authz.RoleViewer))
	assert.NoError(err)
}

func (s *authOnboardingTestSuite) TestAutoOnboardOrganizations() {
	ctx := context.Background()
	t := s.T()
	assert := assert.New(t)

	svc := s.newAuthService("testing-org", "viewer")

	org, err := s.Organization.FindByName(ctx, "testing-org")
	assert.Error(err)
	assert.Nil(org)

	err = autoOnboardOnOrganizations(ctx, svc, s.usr)
	assert.NoError(err)

	org, err = s.Organization.FindByName(ctx, "testing-org")
	assert.NoError(err)
	assert.NotNil(org)

	m, err := s.Membership.FindByOrgAndUser(ctx, org.ID, s.usr.ID)
	assert.NoError(err)
	assert.NotNil(m)
}

func (s *authOnboardingTestSuite) TestAutoOnboardWithExistingMemberships() {
	ctx := context.Background()
	t := s.T()
	assert := assert.New(t)

	svc := s.newAuthService(s.org.Name, string(s.m.Role))

	org, err := s.Organization.FindByName(ctx, s.org.Name)
	assert.NoError(err)
	assert.NotNil(org)

	m, err := s.Membership.FindByOrgAndUser(ctx, org.ID, s.usr1.ID)
	assert.NoError(err)
	assert.NotNil(m)
	assert.Equal(s.m.Role, m.Role)

	err = autoOnboardOnOrganizations(ctx, svc, s.usr1)
	assert.NoError(err)

	newM, err := s.Membership.FindByOrgAndUser(ctx, org.ID, s.usr1.ID)
	assert.NoError(err)
	assert.NotNil(newM)
	assert.Equal(s.m.Role, newM.Role)
}

func (s *authOnboardingTestSuite) newAuthService(orgName, role string) *AuthService {
	return &AuthService{
		onboardingConfig: []*conf.OnboardingSpec{
			{Name: orgName, Role: role},
		},
		userUseCase:       s.TestingUseCases.User,
		orgUseCase:        s.TestingUseCases.Organization,
		membershipUseCase: s.TestingUseCases.Membership,
	}
}
