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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/docker/distribution/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func (s *workflowIntegrationTestSuite) TestChangeVisibility() {
	assert := assert.New(s.T())
	org2, err := s.Organization.Create(context.Background(), "testing org")
	assert.NoError(err)

	s.Run("by default the workflow is non public", func() {
		assert.False(s.workflow.Public)
	})

	testCases := []struct {
		name       string
		workflowID string
		orgID      string
		public     bool
		wantErr    bool
	}{
		{
			name:       "non existing workflow",
			workflowID: uuid.Generate().String(),
			orgID:      s.org.ID,
			wantErr:    true,
		},
		{
			name:       "invalid uuid",
			workflowID: "deadbeef",
			orgID:      s.org.ID,
			wantErr:    true,
		},
		{
			name:       "valid workflow set to true",
			workflowID: s.workflow.ID.String(),
			orgID:      s.org.ID,
			public:     true,
		},
		{
			name:       "valid workflow set to false",
			workflowID: s.workflow.ID.String(),
			orgID:      s.org.ID,
			public:     false,
		},
		{
			name:       "valid workflow in other org",
			workflowID: s.workflow.ID.String(),
			orgID:      org2.ID,
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			wf, err := s.Workflow.ChangeVisibility(context.Background(), tc.orgID, tc.workflowID, tc.public)
			if tc.wantErr {
				assert.Error(err)
				return
			}
			assert.NoError(err)
			assert.Equal(tc.public, wf.Public)
		})
	}
}

// Run the tests
func TestWorkflowUseCase(t *testing.T) {
	suite.Run(t, new(workflowIntegrationTestSuite))
}

// Utility struct to hold the test suite
type workflowIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org      *biz.Organization
	workflow *biz.Workflow
}

func (s *workflowIntegrationTestSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	ctx := context.Background()
	s.org, err = s.Organization.Create(ctx, "testing org")
	assert.NoError(err)
	s.workflow, err = s.Workflow.Create(ctx, &biz.CreateOpts{Name: "test workflow", OrgID: s.org.ID})
	assert.NoError(err)
}
