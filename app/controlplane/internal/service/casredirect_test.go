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
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestCASMappingProjectFilter is a regression test for the cross-project download bypass: a
// project-scoped API token must restrict the CAS-mapping lookup to its own project on every path
// (both CASRedirectService.GetDownloadURL and CASCredentialsService.Get go through this helper).
func TestCASMappingProjectFilter(t *testing.T) {
	orgID := uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	projectID := uuid.MustParse("00000000-0000-0000-0000-0000000000bb")

	testCases := []struct {
		name  string
		token *entities.APIToken
		want  map[uuid.UUID][]uuid.UUID
	}{
		{
			name:  "nil token yields no filter",
			token: nil,
			want:  nil,
		},
		{
			name:  "org-scoped token (nil ProjectID) yields no filter",
			token: &entities.APIToken{},
			want:  nil,
		},
		{
			name:  "project-scoped token restricts to its own project",
			token: &entities.APIToken{ProjectID: &projectID},
			want:  map[uuid.UUID][]uuid.UUID{orgID: {projectID}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := casMappingProjectFilter(orgID, tc.token)
			assert.Equal(t, tc.want, got)
		})
	}
}
