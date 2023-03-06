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

	"github.com/chainloop-dev/bedrock/app/controlplane/internal/biz"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/data/ent"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestEntOCIRepoTo(t *testing.T) {
	testRepo := &ent.OCIRepository{
		ID:         uuid.New(),
		Repo:       "test-repo",
		SecretName: "test-secret",
		CreatedAt:  time.Now(),
	}

	tests := []struct {
		input  *ent.OCIRepository
		output *biz.OCIRepository
	}{
		{nil, nil},
		{testRepo, &biz.OCIRepository{
			ID:         testRepo.ID.String(),
			Repo:       testRepo.Repo,
			SecretName: testRepo.SecretName,
			CreatedAt:  toTimePtr(testRepo.CreatedAt),
		}},
	}

	for _, tc := range tests {
		got := entOCIRepoToBiz(tc.input)
		assert.Equal(t, tc.output, got)
	}
}
