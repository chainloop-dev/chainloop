//
// Copyright 2025 The Chainloop Authors.
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

package events_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/stretchr/testify/require"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestWorkflowEvents(t *testing.T) {
	userUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	apiTokenUUID, err := uuid.Parse("2089bb36-e27b-428b-8009-d015c8737c55")
	require.NoError(t, err)
	orgUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	wfContractName := "test-contract"
	wfContractUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	wfDescription := "test description"
	wfUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	wfName := "test-workflow"
	projectName := "test-project"
	newTeam := "test-team"

	tests := []struct {
		name     string
		event    auditor.LogEntry
		expected string
		actor    auditor.ActorType
		actorID  uuid.UUID
	}{
		{
			name: "Workflow created by user",
			event: &events.WorkflowCreated{
				WorkflowBase: &events.WorkflowBase{
					WorkflowID:   uuidPtr(wfUUID),
					WorkflowName: wfName,
					ProjectName:  projectName,
				},
				WorkflowContractID:   &wfContractUUID,
				WorkflowContractName: wfContractName,
				WorkflowDescription:  &wfDescription,
				Team:                 &newTeam,
				Public:               false,
			},
			expected: "testdata/workflows/workflow_created.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Workflow created by API token",
			event: &events.WorkflowCreated{
				WorkflowBase: &events.WorkflowBase{
					WorkflowID:   uuidPtr(wfUUID),
					WorkflowName: wfName,
					ProjectName:  projectName,
				},
				WorkflowContractID:   &wfContractUUID,
				WorkflowContractName: wfContractName,
				WorkflowDescription:  &wfDescription,
				Team:                 &newTeam,
				Public:               false,
			},
			expected: "testdata/workflows/workflow_created_by_api_token.json",
			actor:    auditor.ActorTypeAPIToken,
			actorID:  apiTokenUUID,
		},
		{
			name: "Workflow updated by user",
			event: &events.WorkflowUpdated{
				WorkflowBase: &events.WorkflowBase{
					WorkflowID:   uuidPtr(wfContractUUID),
					WorkflowName: wfContractName,
					ProjectName:  projectName,
				},
				NewDescription: &wfDescription,
				NewTeam:        &newTeam,
				NewPublic:      boolPtr(true),
			},
			expected: "testdata/workflows/workflow_updated.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Workflow updated by API token",
			event: &events.WorkflowUpdated{
				WorkflowBase: &events.WorkflowBase{
					WorkflowID:   uuidPtr(wfContractUUID),
					WorkflowName: wfContractName,
					ProjectName:  projectName,
				},
				NewDescription: &wfDescription,
				NewTeam:        &newTeam,
				NewPublic:      boolPtr(true),
			},
			expected: "testdata/workflows/workflow_updated_by_api_token.json",
			actor:    auditor.ActorTypeAPIToken,
			actorID:  apiTokenUUID,
		},
		{
			name: "Workflow updated with workflow contract by user",
			event: &events.WorkflowUpdated{
				WorkflowBase: &events.WorkflowBase{
					WorkflowID:   uuidPtr(wfContractUUID),
					WorkflowName: wfContractName,
					ProjectName:  projectName,
				},
				NewDescription:          &wfDescription,
				NewTeam:                 &newTeam,
				NewPublic:               boolPtr(true),
				NewWorkflowContractID:   &wfContractUUID,
				NewWorkflowContractName: &wfContractName,
			},
			expected: "testdata/workflows/workflow_updated_with_workflow_contract.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Workflow updated with workflow contract by API token",
			event: &events.WorkflowUpdated{
				WorkflowBase: &events.WorkflowBase{
					WorkflowID:   uuidPtr(wfContractUUID),
					WorkflowName: wfContractName,
					ProjectName:  projectName,
				},
				NewDescription:          &wfDescription,
				NewTeam:                 &newTeam,
				NewPublic:               boolPtr(true),
				NewWorkflowContractID:   &wfContractUUID,
				NewWorkflowContractName: &wfContractName,
			},
			expected: "testdata/workflows/workflow_updated_with_workflow_contract_by_api_token.json",
			actor:    auditor.ActorTypeAPIToken,
			actorID:  userUUID,
		},
		{
			name: "Workflow deleted by user",
			event: &events.WorkflowDeleted{
				WorkflowBase: &events.WorkflowBase{
					WorkflowID:   uuidPtr(wfContractUUID),
					WorkflowName: wfContractName,
					ProjectName:  projectName,
				},
			},
			expected: "testdata/workflows/workflow_deleted.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Workflow deleted by API token",
			event: &events.WorkflowDeleted{
				WorkflowBase: &events.WorkflowBase{
					WorkflowID:   uuidPtr(wfContractUUID),
					WorkflowName: wfContractName,
					ProjectName:  projectName,
				},
			},
			expected: "testdata/workflows/workflow_deleted_by_api_token.json",
			actor:    auditor.ActorTypeAPIToken,
			actorID:  apiTokenUUID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []auditor.GeneratorOption{
				auditor.WithOrgID(orgUUID),
			}
			if tt.actor == auditor.ActorTypeAPIToken {
				opts = append(opts, auditor.WithActor(auditor.ActorTypeAPIToken, tt.actorID, ""))
			} else {
				opts = append(opts, auditor.WithActor(auditor.ActorTypeUser, tt.actorID, testEmail))
			}

			eventPayload, err := auditor.GenerateAuditEvent(tt.event, opts...)
			require.NoError(t, err)

			want, err := json.MarshalIndent(eventPayload.Data, "", "  ")
			require.NoError(t, err)

			if updateGolden {
				err := os.WriteFile(filepath.Clean(tt.expected), want, 0600)
				require.NoError(t, err)
			}

			gotRaw, err := os.ReadFile(filepath.Clean(tt.expected))
			require.NoError(t, err)

			var gotPayload auditor.AuditEventPayload
			err = json.Unmarshal(gotRaw, &gotPayload)
			require.NoError(t, err)
			got, err := json.MarshalIndent(gotPayload, "", "  ")
			require.NoError(t, err)

			assert.Equal(t, string(want), string(got))
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}
