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

// stubWorkflowRunRepo returns a canned creation result. Only Create is
// exercised; any other call panics on the nil embedded interface.
type stubWorkflowRunRepo struct {
	WorkflowRunRepo
	result *WorkflowRunRepoCreateResult
}

func (s *stubWorkflowRunRepo) Create(_ context.Context, _ *WorkflowRunRepoCreateOpts) (*WorkflowRunRepoCreateResult, error) {
	return s.result, nil
}

// A project version only reaches "latest" through a creation or a promotion,
// and the products that track the latest version of a project reconcile off
// these audit events, so both transitions must be reported.
func TestWorkflowRunCreateDispatchesProjectVersionEvents(t *testing.T) {
	project := &Project{ID: uuid.New(), Name: "test-project", OrgID: uuid.New()}
	version := &ProjectVersion{ID: uuid.New(), Version: "v1.0.0", Prerelease: true, Latest: true}

	testCases := []struct {
		name               string
		versionCreated     bool
		versionPromoted    bool
		wantAction         string // empty means nothing should be dispatched
		wantMarkedAsLatest bool
	}{
		{
			name:           "version created",
			versionCreated: true,
			wantAction:     events.ProjectVersionCreatedActionType,
		},
		{
			name:               "existing version promoted to latest",
			versionPromoted:    true,
			wantAction:         events.ProjectVersionUpdatedActionType,
			wantMarkedAsLatest: true,
		},
		{
			name: "version neither created nor promoted",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			auditorUC, publisher := newRecordingAuditor()
			repo := &stubWorkflowRunRepo{result: &WorkflowRunRepoCreateResult{
				Project:         project,
				Run:             &WorkflowRun{ID: uuid.New(), ProjectVersion: version},
				VersionCreated:  tc.versionCreated,
				VersionPromoted: tc.versionPromoted,
			}}

			uc, err := NewWorkflowRunUseCase(&WorkflowRunUseCaseOpts{WfrRepo: repo, AuditorUC: auditorUC})
			require.NoError(t, err)

			_, err = uc.Create(ctxWithAPITokenActor(context.Background()), &WorkflowRunCreateOpts{
				WorkflowID:   uuid.NewString(),
				CASBackendID: uuid.New(),
				ContractRevision: &WorkflowContractWithVersion{
					Contract: &WorkflowContract{LatestRevision: 1},
					Version:  &WorkflowContractVersion{ID: uuid.New(), Revision: 1},
				},
				ProjectVersion: version.Version,
			})
			require.NoError(t, err)

			if tc.wantAction == "" {
				require.Empty(t, publisher.published)
				return
			}

			publisher.assertSingleProjectVersionEvent(t, tc.wantAction, project, version, tc.wantMarkedAsLatest)
		})
	}
}
