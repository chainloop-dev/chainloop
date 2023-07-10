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

package usercontext

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestShouldRevalidate(t *testing.T) {
	testCases := []struct {
		name            string
		repoStatus      biz.OCIRepoValidationStatus
		repoValidatedAt time.Time
		expected        bool
	}{
		{
			name:            "should revalidate if status is not ok and new",
			repoStatus:      biz.OCIRepoValidationFailed,
			repoValidatedAt: time.Now(),
			expected:        true,
		},
		{
			name:            "should revalidate if status is not ok and old",
			repoStatus:      biz.OCIRepoValidationFailed,
			repoValidatedAt: time.Now().Add(-2 * validationTimeOffset),
			expected:        true,
		},
		{
			name:            "should revalidate if status is ok but validated at is too old",
			repoStatus:      biz.OCIRepoValidationOK,
			repoValidatedAt: time.Now().Add(-2 * validationTimeOffset),
			expected:        true,
		},
		{
			name:            "should not revalidate if status is ok and validated at is recent",
			repoStatus:      biz.OCIRepoValidationOK,
			repoValidatedAt: time.Now().Add(-(validationTimeOffset - time.Second)),
			expected:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &biz.CASBackend{
				ValidationStatus: tc.repoStatus,
				ValidatedAt:      &tc.repoValidatedAt,
			}

			assert.Equal(t, tc.expected, shouldRevalidate(repo))
		})
	}
}

func TestValidateRepo(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)
	repo := &biz.CASBackend{ID: uuid.NewString(), ValidatedAt: toTimePtr(time.Now())}

	t.Run("validation error", func(t *testing.T) {
		useCase := mocks.NewOCIRepositoryReader(t)
		useCase.On("PerformValidation", ctx, repo.ID).Return(errors.New("validation error"))
		got, err := validateRepo(ctx, useCase, repo)
		assert.Error(err)
		assert.Nil(got)
	})

	t.Run("validation ok, returns updated repo", func(t *testing.T) {
		useCase := mocks.NewOCIRepositoryReader(t)
		useCase.On("PerformValidation", ctx, repo.ID).Return(nil)

		want := &biz.CASBackend{ID: repo.ID, ValidatedAt: toTimePtr(time.Now())}
		useCase.On("FindByID", ctx, repo.ID).Return(want, nil)
		got, err := validateRepo(ctx, useCase, repo)
		assert.NoError(err)
		assert.Equal(want, got)
	})
}

func toTimePtr(t time.Time) *time.Time {
	return &t
}
