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
	"errors"
	"io"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	repoM "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type workflowrunExpirerTestSuite struct {
	suite.Suite
	useCase        *biz.WorkflowRunExpirerUseCase
	repo           *repoM.WorkflowRunRepo
	prometheusRepo *repoM.PromObservable
	ctx            context.Context
	err            error
	toExpire       []*biz.WorkflowRun
	threshold      time.Time
}

type workflowrunTestSuite struct {
	suite.Suite
	useCase *biz.WorkflowRunUseCase
	repo    *repoM.WorkflowRunRepo
	validID uuid.UUID
}

func (s *workflowrunTestSuite) SetupTest() {
	s.repo = repoM.NewWorkflowRunRepo(s.T())
	uc, err := biz.NewWorkflowRunUseCase(s.repo, nil, nil)
	require.NoError(s.T(), err)
	s.useCase = uc
	s.validID = uuid.New()
}

func (s *workflowrunExpirerTestSuite) SetupTest() {
	now := time.Now()

	s.repo = repoM.NewWorkflowRunRepo(s.T())
	s.prometheusRepo = repoM.NewPromObservable(s.T())
	s.useCase = biz.NewWorkflowRunExpirerUseCase(s.repo, s.prometheusRepo, log.NewStdLogger(io.Discard))
	s.ctx = context.TODO()
	s.err = errors.New("an error")
	s.threshold = now
	s.toExpire = []*biz.WorkflowRun{
		{ID: uuid.New(), CreatedAt: &now}, {ID: uuid.New(), CreatedAt: &now},
	}
}

func (s *workflowrunExpirerTestSuite) TestSweepListError() {
	assert := assert.New(s.T())

	s.repo.On("ListNotFinishedOlderThan", s.ctx, s.threshold).Return(nil, s.err)
	err := s.useCase.ExpirationSweep(s.ctx, s.threshold)
	assert.ErrorIs(s.err, err)
}

func (s *workflowrunExpirerTestSuite) TestSweepExpireError() {
	assert := assert.New(s.T())

	s.repo.On("ListNotFinishedOlderThan", s.ctx, s.threshold).Return(s.toExpire, nil)
	s.repo.On("Expire", s.ctx, s.toExpire[0].ID).Return(s.err)
	err := s.useCase.ExpirationSweep(s.ctx, s.threshold)
	assert.Error(err)
}
func (s *workflowrunExpirerTestSuite) TestSweepExpireOK() {
	assert := assert.New(s.T())

	s.repo.On("ListNotFinishedOlderThan", s.ctx, s.threshold).Return(s.toExpire, nil)

	s.repo.On("Expire", s.ctx, s.toExpire[0].ID).Return(nil)
	s.repo.On("Expire", s.ctx, s.toExpire[1].ID).Return(nil)
	s.prometheusRepo.On("ObserveAttestationIfNeeded", s.ctx, s.toExpire[0], biz.WorkflowRunExpired).Return(true)
	s.prometheusRepo.On("ObserveAttestationIfNeeded", s.ctx, s.toExpire[1], biz.WorkflowRunExpired).Return(true)

	err := s.useCase.ExpirationSweep(s.ctx, s.threshold)
	assert.NoError(err)
}

func (s *workflowrunTestSuite) TestMarkAsFinished() {
	testCases := []struct {
		name      string
		id        string
		status    biz.WorkflowRunStatus
		reason    string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "invalid ID",
			id:        "invalid",
			expectErr: true,
			errMsg:    "invalid UUID length: 7",
		},
		{
			name:   "set to success",
			id:     s.validID.String(),
			status: biz.WorkflowRunSuccess,
		},
		{
			name:   "set to error",
			id:     s.validID.String(),
			status: biz.WorkflowRunError,
			reason: "trigger type failure",
		},
		{
			name:   "set to cancelled",
			id:     s.validID.String(),
			status: biz.WorkflowRunCancelled,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			if !tc.expectErr {
				uuid, err := uuid.Parse(tc.id)
				require.NoError(t, err)

				s.repo.On("MarkAsFinished", mock.Anything, uuid, tc.status, tc.reason).Return(nil)
			}

			// s.repo.On("MarkAsFinished", s.ctx, id, tc.wantStatus, tc.reason).Return(tc.err)
			err := s.useCase.MarkAsFinished(context.Background(), tc.id, tc.status, tc.reason)
			if tc.expectErr {
				assert.Error(err)
				assert.ErrorContains(err, tc.errMsg)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestWorkflorRunExpirer(t *testing.T) {
	suite.Run(t, new(workflowrunExpirerTestSuite))
}

func TestWorkflorRun(t *testing.T) {
	suite.Run(t, new(workflowrunTestSuite))
}
