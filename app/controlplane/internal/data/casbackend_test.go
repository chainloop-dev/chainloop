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

package data

import (
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestEntCASBackendTo(t *testing.T) {
	testRepo := &ent.CASBackend{
		ID:         uuid.New(),
		Name:       "test-repo",
		Provider:   "test-provider",
		SecretName: "test-secret",
		CreatedAt:  time.Now(),
	}

	tests := []struct {
		input  *ent.CASBackend
		output *biz.CASBackend
	}{
		{nil, nil},
		{testRepo, &biz.CASBackend{
			ID:         testRepo.ID.String(),
			Name:       testRepo.Name,
			SecretName: testRepo.SecretName,
			CreatedAt:  toTimePtr(testRepo.CreatedAt),
			Provider:   biz.CASBackendProvider(testRepo.Provider),
		}},
	}

	for _, tc := range tests {
		got := entCASBackendToBiz(tc.input)
		assert.Equal(t, tc.output, got)
	}
}
