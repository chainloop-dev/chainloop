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
	"io"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	repoM "github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type organizationTestSuite struct {
	suite.Suite
}

func (s *organizationTestSuite) TestCreateWithRandomName() {
	repo := repoM.NewOrganizationRepo(s.T())
	uc := biz.NewOrganizationUseCase(repo, nil, nil, nil, log.NewStdLogger(io.Discard))

	s.Run("the org exists, we retry", func() {
		ctx := context.Background()
		// the first one fails because it already exists
		repo.On("Create", ctx, mock.Anything).Once().Return(nil, biz.ErrAlreadyExists)
		// but the second call creates the org
		repo.On("Create", ctx, mock.Anything).Once().Return(&biz.Organization{Name: "foobar"}, nil)
		got, err := uc.CreateWithRandomName(ctx)
		s.NoError(err)
		s.Equal("foobar", got.Name)
	})

	s.Run("if it runs out of tries, it fails", func() {
		ctx := context.Background()
		// the first one fails because it already exists
		repo.On("Create", ctx, mock.Anything).Times(biz.RandomNameMaxTries).Return(nil, biz.ErrAlreadyExists)
		got, err := uc.CreateWithRandomName(ctx)
		s.Error(err)
		s.Nil(got)
	})
}

func (s *organizationTestSuite) TestValidateOrgName() {
	testCases := []struct {
		name          string
		expectedError bool
	}{
		{"", true},
		{"a", false},
		{"aa-aa", false},
		{"-aaa", true},
		// no under-scores
		{"aaa_aaa", true},
		{"1-aaaa", false},
		{"Aaaaa", true},
		{"12-foo-bar-waz", false},
		// 63 max
		{"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijk", false},
		// over the max size
		{"aabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijk", true},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			err := biz.ValidateIsDNS1123(tc.name)
			if tc.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

// Run all the tests
func TestOrganization(t *testing.T) {
	suite.Run(t, new(organizationTestSuite))
}
