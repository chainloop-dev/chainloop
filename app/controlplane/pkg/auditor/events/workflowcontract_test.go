//
// Copyright 2024 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContractEvents(t *testing.T) {
	userUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	apiTokenUUID, err := uuid.Parse("2089bb36-e27b-428b-8009-d015c8737c55")
	require.NoError(t, err)
	orgUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	wfContractName := "test-contract"
	wfContractUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	wfContractRevisionUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	revisionNumber := 1
	contractDescription := "test description"
	wfUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	wfName := "test-workflow"

	tests := []struct {
		name     string
		event    auditor.LogEntry
		expected string
		actor    auditor.ActorType
		actorID  uuid.UUID
	}{
		{
			name: "Workflow contract created by user",
			event: &events.WorkflowContractCreated{
				WorkflowContractBase: &events.WorkflowContractBase{
					WorkflowContractID:   uuidPtr(wfContractUUID),
					WorkflowContractName: wfContractName,
				},
			},
			expected: "testdata/workflowcontracts/workflow_contract_created.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Workflow contract created by API token",
			event: &events.WorkflowContractCreated{
				WorkflowContractBase: &events.WorkflowContractBase{
					WorkflowContractID:   uuidPtr(wfContractUUID),
					WorkflowContractName: wfContractName,
				},
			},
			expected: "testdata/workflowcontracts/workflow_contract_created_by_api_token.json",
			actor:    auditor.ActorTypeAPIToken,
			actorID:  apiTokenUUID,
		},
		{
			name: "Workflow contract updated by user",
			event: &events.WorkflowContractUpdated{
				WorkflowContractBase: &events.WorkflowContractBase{
					WorkflowContractID:   uuidPtr(wfContractUUID),
					WorkflowContractName: wfContractName,
				},
				NewRevisionID:  &wfContractRevisionUUID,
				NewRevision:    &revisionNumber,
				NewDescription: &contractDescription,
			},
			expected: "testdata/workflowcontracts/workflow_contract_updated.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Workflow contract updated by API token",
			event: &events.WorkflowContractUpdated{
				WorkflowContractBase: &events.WorkflowContractBase{
					WorkflowContractID:   uuidPtr(wfContractUUID),
					WorkflowContractName: wfContractName,
				},
				NewRevisionID:  &wfContractRevisionUUID,
				NewRevision:    &revisionNumber,
				NewDescription: &contractDescription,
			},
			expected: "testdata/workflowcontracts/workflow_contract_updated_by_api_token.json",
			actor:    auditor.ActorTypeAPIToken,
			actorID:  apiTokenUUID,
		},
		{
			name: "Workflow contract deleted by user",
			event: &events.WorkflowContractDeleted{
				WorkflowContractBase: &events.WorkflowContractBase{
					WorkflowContractID:   uuidPtr(wfContractUUID),
					WorkflowContractName: wfContractName,
				},
			},
			expected: "testdata/workflowcontracts/workflow_contract_deleted.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Workflow contract deleted by API token",
			event: &events.WorkflowContractDeleted{
				WorkflowContractBase: &events.WorkflowContractBase{
					WorkflowContractID:   uuidPtr(wfContractUUID),
					WorkflowContractName: wfContractName,
				},
			},
			expected: "testdata/workflowcontracts/workflow_contract_deleted_by_api_token.json",
			actor:    auditor.ActorTypeAPIToken,
			actorID:  apiTokenUUID,
		},
		{
			name: "Workflow attached to contract by user",
			event: &events.WorkflowContractAttached{
				WorkflowContractBase: &events.WorkflowContractBase{
					WorkflowContractID:   uuidPtr(wfContractUUID),
					WorkflowContractName: wfContractName,
				},
				WorkflowID:   uuidPtr(wfUUID),
				WorkflowName: wfName,
			},
			expected: "testdata/workflowcontracts/workflow_attached_to_contract.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Workflow attached to contract by API token",
			event: &events.WorkflowContractAttached{
				WorkflowContractBase: &events.WorkflowContractBase{
					WorkflowContractID:   uuidPtr(wfContractUUID),
					WorkflowContractName: wfContractName,
				},
				WorkflowID:   uuidPtr(wfUUID),
				WorkflowName: wfName,
			},
			expected: "testdata/workflowcontracts/workflow_attached_to_contract_by_api_token.json",
			actor:    auditor.ActorTypeAPIToken,
			actorID:  apiTokenUUID,
		},
		{
			name: "Workflow detached from contract by user",
			event: &events.WorkflowContractDetached{
				WorkflowContractBase: &events.WorkflowContractBase{
					WorkflowContractID:   uuidPtr(wfContractUUID),
					WorkflowContractName: wfContractName,
				},
				WorkflowID:   uuidPtr(wfUUID),
				WorkflowName: wfName,
			},
			expected: "testdata/workflowcontracts/workflow_detached_from_contract.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Workflow detached from contract by API token",
			event: &events.WorkflowContractDetached{
				WorkflowContractBase: &events.WorkflowContractBase{
					WorkflowContractID:   uuidPtr(wfContractUUID),
					WorkflowContractName: wfContractName,
				},
				WorkflowID:   uuidPtr(wfUUID),
				WorkflowName: wfName,
			},
			expected: "testdata/workflowcontracts/workflow_detached_from_contract_by_api_token.json",
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
