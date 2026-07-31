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

package biz

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// stubProjectVersionRepo returns a canned promotion outcome. Only MarkAsLatest
// is exercised; any other call panics on the nil embedded interface.
type stubProjectVersionRepo struct {
	ProjectVersionRepo
	promotion *ProjectVersionPromotion
}

func (s *stubProjectVersionRepo) MarkAsLatest(_ context.Context, _, _ uuid.UUID) (*ProjectVersionPromotion, error) {
	return s.promotion, nil
}

// Promoting a version out of band must be as loud as promoting it through an
// attestation, so that whatever tracks the latest version of a project follows.
func TestMarkAsLatestDispatchesProjectVersionUpdated(t *testing.T) {
	project := &Project{ID: uuid.New(), Name: "test-project", OrgID: uuid.New()}
	version := &ProjectVersion{ID: uuid.New(), Version: "v1.0.0", Prerelease: true, Latest: true}

	testCases := []struct {
		name       string
		promoted   bool
		wantAction string // empty means nothing should be dispatched
	}{
		{
			name:       "version promoted",
			promoted:   true,
			wantAction: events.ProjectVersionUpdatedActionType,
		},
		{
			name: "version already was the latest",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			auditorUC, publisher := newRecordingAuditor()
			repo := &stubProjectVersionRepo{promotion: &ProjectVersionPromotion{
				Promoted: tc.promoted,
				Version:  version,
				Project:  project,
			}}

			uc := NewProjectVersionUseCase(repo, auditorUC, nil)
			err := uc.MarkAsLatest(ctxWithAPITokenActor(context.Background()), project.ID.String(), version.ID.String())
			require.NoError(t, err)

			if tc.wantAction == "" {
				require.Empty(t, publisher.published)
				return
			}

			publisher.assertSingleProjectVersionEvent(t, tc.wantAction, project, version, true)
		})
	}
}
