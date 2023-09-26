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
		repoStatus      biz.CASBackendValidationStatus
		repoValidatedAt time.Time
		expected        bool
	}{
		{
			name:            "should revalidate if status is not ok and new",
			repoStatus:      biz.CASBackendValidationFailed,
			repoValidatedAt: time.Now(),
			expected:        true,
		},
		{
			name:            "should revalidate if status is not ok and old",
			repoStatus:      biz.CASBackendValidationFailed,
			repoValidatedAt: time.Now().Add(-2 * validationTimeOffset),
			expected:        true,
		},
		{
			name:            "should revalidate if status is ok but validated at is too old",
			repoStatus:      biz.CASBackendValidationOK,
			repoValidatedAt: time.Now().Add(-2 * validationTimeOffset),
			expected:        true,
		},
		{
			name:            "should not revalidate if status is ok and validated at is recent",
			repoStatus:      biz.CASBackendValidationOK,
			repoValidatedAt: time.Now().Add(-(validationTimeOffset - time.Second)),
			expected:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &biz.CASBackend{
				ValidationStatus: tc.repoStatus,
				ValidatedAt:      toTimePtr(tc.repoValidatedAt),
			}

			assert.Equal(t, tc.expected, shouldRevalidate(repo))
		})
	}
}

func TestValidateCASBackend(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)
	repo := &biz.CASBackend{ID: uuid.New(), OrganizationID: uuid.New(), ValidatedAt: toTimePtr(time.Now())}

	t.Run("validation error", func(t *testing.T) {
		useCase := mocks.NewCASBackendReader(t)
		useCase.On("PerformValidation", ctx, repo.ID.String()).Return(errors.New("validation error"))
		got, err := validateCASBackend(ctx, useCase, repo)
		assert.Error(err)
		assert.Nil(got)
	})

	t.Run("validation ok, returns updated repo", func(t *testing.T) {
		useCase := mocks.NewCASBackendReader(t)
		useCase.On("PerformValidation", ctx, repo.ID.String()).Return(nil)

		want := &biz.CASBackend{ID: repo.ID, ValidatedAt: toTimePtr(time.Now())}
		useCase.On("FindByIDInOrg", ctx, repo.OrganizationID.String(), repo.ID.String()).Return(want, nil)
		got, err := validateCASBackend(ctx, useCase, repo)
		assert.NoError(err)
		assert.Equal(want, got)
	})
}

func toTimePtr(t time.Time) *time.Time {
	return &t
}
