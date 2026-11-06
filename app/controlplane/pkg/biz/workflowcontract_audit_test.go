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
	"encoding/json"
	"io"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubWorkflowContractRepo returns the contract before and after an update.
// Only FindByNameInOrg and Update are exercised; any other call panics on the
// nil embedded interface.
type stubWorkflowContractRepo struct {
	WorkflowContractRepo
	before, after *WorkflowContract
}

func (s *stubWorkflowContractRepo) FindByNameInOrg(_ context.Context, _ uuid.UUID, _ string) (*WorkflowContract, error) {
	return s.before, nil
}

func (s *stubWorkflowContractRepo) Update(_ context.Context, _ uuid.UUID, _ string, _ *ContractUpdateOpts) (*WorkflowContractWithVersion, error) {
	return &WorkflowContractWithVersion{
		Contract: s.after,
		Version:  &WorkflowContractVersion{ID: uuid.New(), Revision: s.after.LatestRevision},
	}, nil
}

// Spec issue-3644 R-008: each conversion writes an audit event with the old and the new level.
func TestWorkflowContractUpdateDispatchesScopeChanged(t *testing.T) {
	project := &ScopedEntity{Type: string(ContractScopeProject), ID: uuid.New(), Name: "my-project"}
	orgScope := &events.WorkflowContractScope{Type: events.WorkflowContractScopeOrganization}
	projectScope := &events.WorkflowContractScope{Type: events.WorkflowContractScopeProject, ProjectID: &project.ID, ProjectName: project.Name}

	testCases := []struct {
		name          string
		before, after *ScopedEntity
		scope         *ContractScopeChange
		// nil means no scope event is dispatched
		wantOld, wantNew *events.WorkflowContractScope
	}{
		{
			name:    "organization to project",
			after:   project,
			scope:   &ContractScopeChange{ProjectID: &project.ID},
			wantOld: orgScope,
			wantNew: projectScope,
		},
		{
			name:    "project to organization",
			before:  project,
			scope:   &ContractScopeChange{},
			wantOld: projectScope,
			wantNew: orgScope,
		},
		{
			name:   "same level",
			before: project,
			after:  project,
			scope:  &ContractScopeChange{ProjectID: &project.ID},
		},
		{
			name:   "no level in the request",
			before: project,
			after:  project,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			auditorUC, publisher := newRecordingAuditor()
			contractID := uuid.New()
			repo := &stubWorkflowContractRepo{
				before: &WorkflowContract{ID: contractID, Name: "my-contract", LatestRevision: 1, ScopedEntity: cloneScope(tc.before)},
				after:  &WorkflowContract{ID: contractID, Name: "my-contract", LatestRevision: 1, ScopedEntity: cloneScope(tc.after)},
			}
			uc := NewWorkflowContractUseCase(repo, nil, auditorUC, log.NewStdLogger(io.Discard))

			_, err := uc.Update(ctxWithAPITokenActor(context.Background()), uuid.NewString(), "my-contract", &WorkflowContractUpdateOpts{Scope: tc.scope})
			require.NoError(t, err)

			var scopeEvents []json.RawMessage
			for _, p := range publisher.published {
				if p.Data.ActionType == events.WorkflowContractScopeChangedActionType {
					assert.Equal(t, &contractID, p.Data.TargetID)
					scopeEvents = append(scopeEvents, p.Data.Info)
				}
			}

			if tc.wantNew == nil {
				assert.Empty(t, scopeEvents)
				return
			}

			require.Len(t, scopeEvents, 1)
			var info struct {
				OldScope *events.WorkflowContractScope `json:"old_scope"`
				NewScope *events.WorkflowContractScope `json:"new_scope"`
			}
			require.NoError(t, json.Unmarshal(scopeEvents[0], &info))
			assert.Equal(t, tc.wantOld, info.OldScope)
			assert.Equal(t, tc.wantNew, info.NewScope)
		})
	}
}

// cloneScope returns a copy, as the repository returns a new object on each call.
func cloneScope(s *ScopedEntity) *ScopedEntity {
	if s == nil {
		return nil
	}

	c := *s
	return &c
}
