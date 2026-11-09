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

// stubWorkflowContractRepo returns the contract that the use case reads before the update,
// and the result of the update. Only FindByNameInOrg and Update are exercised; any other
// call panics on the nil embedded interface.
type stubWorkflowContractRepo struct {
	WorkflowContractRepo
	preRead *WorkflowContract
	result  *WorkflowContractWithVersion
}

func (s *stubWorkflowContractRepo) FindByNameInOrg(_ context.Context, _ uuid.UUID, _ string) (*WorkflowContract, error) {
	return s.preRead, nil
}

func (s *stubWorkflowContractRepo) Update(_ context.Context, _ uuid.UUID, _ string, _ *ContractUpdateOpts) (*WorkflowContractWithVersion, error) {
	return s.result, nil
}

// Spec issue-3644 R-008: each conversion writes an audit event with the old and the new level.
// Both levels are the ones that the repository saw under the contract lock, not the one read
// before the update or the one read after the commit, which a concurrent conversion can make stale.
func TestWorkflowContractUpdateDispatchesScopeChanged(t *testing.T) {
	project := &ScopedEntity{Type: string(ContractScopeProject), ID: uuid.New(), Name: "my-project"}
	otherProject := &ScopedEntity{Type: string(ContractScopeProject), ID: uuid.New(), Name: "other-project"}
	orgScope := &events.WorkflowContractScope{Type: events.WorkflowContractScopeOrganization}
	projectScope := &events.WorkflowContractScope{Type: events.WorkflowContractScopeProject, ProjectID: &project.ID, ProjectName: project.Name}
	otherProjectScope := &events.WorkflowContractScope{Type: events.WorkflowContractScopeProject, ProjectID: &otherProject.ID, ProjectName: otherProject.Name}

	testCases := []struct {
		name string
		// preRead is the level that the use case reads before the update
		preRead *ScopedEntity
		// after is the level of the returned contract, read after the commit
		after *ScopedEntity
		// change is the change of level that the repository reports
		change *ContractScopeTransition
		// nil means no scope event is dispatched
		wantOld, wantNew *events.WorkflowContractScope
	}{
		{
			name:    "organization to project",
			after:   project,
			change:  &ContractScopeTransition{To: project},
			wantOld: orgScope,
			wantNew: projectScope,
		},
		{
			name:    "project to organization",
			preRead: project,
			change:  &ContractScopeTransition{From: project},
			wantOld: projectScope,
			wantNew: orgScope,
		},
		{
			name:    "the repository did not change the level",
			preRead: project,
			after:   project,
		},
		{
			name:    "the old level is the one the repository saw, not the stale pre-read",
			change:  &ContractScopeTransition{From: otherProject},
			wantOld: otherProjectScope,
			wantNew: orgScope,
		},
		{
			name:    "the new level is the one the repository set, not the read after the commit",
			after:   otherProject,
			change:  &ContractScopeTransition{To: project},
			wantOld: orgScope,
			wantNew: projectScope,
		},
		{
			name:  "no event when a concurrent conversion already set the level",
			after: project,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			auditorUC, publisher := newRecordingAuditor()
			contractID := uuid.New()
			repo := &stubWorkflowContractRepo{
				preRead: &WorkflowContract{ID: contractID, Name: "my-contract", LatestRevision: 1, ScopedEntity: tc.preRead},
				result: &WorkflowContractWithVersion{
					Contract:    &WorkflowContract{ID: contractID, Name: "my-contract", LatestRevision: 1, ScopedEntity: tc.after},
					Version:     &WorkflowContractVersion{ID: uuid.New(), Revision: 1},
					ScopeChange: tc.change,
				},
			}
			uc := NewWorkflowContractUseCase(repo, nil, auditorUC, log.NewStdLogger(io.Discard))

			_, err := uc.Update(ctxWithAPITokenActor(context.Background()), uuid.NewString(), "my-contract", &WorkflowContractUpdateOpts{Scope: &ContractScopeChange{}})
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
