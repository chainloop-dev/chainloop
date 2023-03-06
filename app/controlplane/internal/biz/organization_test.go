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

package biz_test

import (
	"context"
	"testing"

	"github.com/chainloop-dev/bedrock/app/controlplane/internal/biz"
	repoM "github.com/chainloop-dev/bedrock/app/controlplane/internal/biz/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type organizationTestSuite struct {
	suite.Suite
	repo    *repoM.OrganizationRepo
	useCase *biz.OrganizationUseCase
}

func (s *organizationTestSuite) SetupTest() {
	s.repo = repoM.NewOrganizationRepo(s.T())
	s.useCase = biz.NewOrganizationUsecase(s.repo, nil, nil, nil)
}

func (s *organizationTestSuite) TestCreate() {
	assert := assert.New(s.T())
	ctx := context.Background()
	tests := []struct {
		name string
	}{{"defined"}, {""}}

	newOrg := &biz.Organization{}
	s.repo.On("Create", ctx, mock.AnythingOfType("string")).Return(
		func(ctx context.Context, s string) *biz.Organization {
			newOrg.Name = s
			return newOrg
		}, nil,
	)

	for _, tc := range tests {
		gotOrg, err := s.useCase.Create(ctx, tc.name)
		assert.NoError(err)
		// The name was provided
		if tc.name != "" {
			assert.Equal(gotOrg.Name, tc.name)
		}
		// The name is always set, even if it was not provided
		assert.NotEmpty(gotOrg.Name)
	}
}

// Run all the tests
func TestOrganization(t *testing.T) {
	suite.Run(t, new(organizationTestSuite))
}
