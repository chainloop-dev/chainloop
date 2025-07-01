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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectEvents(t *testing.T) {
	userUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	orgUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	projectUUID, err := uuid.Parse("3089bb36-e27b-428b-8009-d015c8737c56")
	require.NoError(t, err)
	memberUUID, err := uuid.Parse("4089bb36-e27b-428b-8009-d015c8737c57")
	require.NoError(t, err)
	projectName := "test-project"
	userEmail := "test@example.com"

	tests := []struct {
		name     string
		event    auditor.LogEntry
		expected string
		actor    auditor.ActorType
		actorID  uuid.UUID
	}{
		{
			name: "ProjectMemberAdded",
			event: &events.ProjectMemberAdded{
				ProjectBase: &events.ProjectBase{
					ProjectID:   &projectUUID,
					ProjectName: projectName,
				},
				UserID:    &memberUUID,
				UserEmail: userEmail,
				Role:      string(authz.RoleProjectViewer),
			},
			expected: "testdata/projects/project_member_added.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "ProjectMemberAdded as admin",
			event: &events.ProjectMemberAdded{
				ProjectBase: &events.ProjectBase{
					ProjectID:   &projectUUID,
					ProjectName: projectName,
				},
				UserID:    &memberUUID,
				UserEmail: userEmail,
				Role:      "role:project:admin",
			},
			expected: "testdata/projects/project_member_added_as_admin.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "ProjectMemberAdded with API Token",
			event: &events.ProjectMemberAdded{
				ProjectBase: &events.ProjectBase{
					ProjectID:   &projectUUID,
					ProjectName: projectName,
				},
				UserID:    &memberUUID,
				UserEmail: userEmail,
				Role:      string(authz.RoleProjectViewer),
			},
			expected: "testdata/projects/project_member_added_with_api_token.json",
			actor:    auditor.ActorTypeAPIToken,
			actorID:  userUUID,
		},
		{
			name: "ProjectMemberRemoved",
			event: &events.ProjectMemberRemoved{
				ProjectBase: &events.ProjectBase{
					ProjectID:   &projectUUID,
					ProjectName: projectName,
				},
				UserID:    &memberUUID,
				UserEmail: userEmail,
			},
			expected: "testdata/projects/project_member_removed.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "ProjectMemberRemoved with API Token",
			event: &events.ProjectMemberRemoved{
				ProjectBase: &events.ProjectBase{
					ProjectID:   &projectUUID,
					ProjectName: projectName,
				},
				UserID:    &memberUUID,
				UserEmail: userEmail,
			},
			expected: "testdata/projects/project_member_removed_with_api_token.json",
			actor:    auditor.ActorTypeAPIToken,
			actorID:  userUUID,
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
				err := os.MkdirAll(filepath.Dir(tt.expected), 0755)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Clean(tt.expected), want, 0600)
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

// TestProjectEventsFailed tests the behavior of project events when they are expected to fail
func TestProjectEventsFailed(t *testing.T) {
	projectUUID, err := uuid.Parse("3089bb36-e27b-428b-8009-d015c8737c56")
	require.NoError(t, err)
	memberUUID, err := uuid.Parse("4089bb36-e27b-428b-8009-d015c8737c57")
	require.NoError(t, err)

	tests := []struct {
		name        string
		event       auditor.LogEntry
		expectedErr string
	}{
		{
			name: "Project member added with missing ProjectID",
			event: &events.ProjectMemberAdded{
				ProjectBase: &events.ProjectBase{
					ProjectName: "test-project",
				},
				UserID:    &memberUUID,
				UserEmail: "test@example.com",
				Role:      "admin",
			},
			expectedErr: "project id and name are required",
		},
		{
			name: "Project member added with missing ProjectName",
			event: &events.ProjectMemberAdded{
				ProjectBase: &events.ProjectBase{
					ProjectID: &projectUUID,
				},
				UserID:    &memberUUID,
				UserEmail: "test@example.com",
				Role:      "admin",
			},
			expectedErr: "project id and name are required",
		},
		{
			name: "Project member added with missing UserID",
			event: &events.ProjectMemberAdded{
				ProjectBase: &events.ProjectBase{
					ProjectID:   &projectUUID,
					ProjectName: "test-project",
				},
				UserEmail: "test@example.com",
				Role:      "admin",
			},
			expectedErr: "user ID is required",
		},
		{
			name: "Project member removed with missing ProjectID",
			event: &events.ProjectMemberRemoved{
				ProjectBase: &events.ProjectBase{
					ProjectName: "test-project",
				},
				UserID:    &memberUUID,
				UserEmail: "test@example.com",
			},
			expectedErr: "project id and name are required",
		},
		{
			name: "Project member removed with missing ProjectName",
			event: &events.ProjectMemberRemoved{
				ProjectBase: &events.ProjectBase{
					ProjectID: &projectUUID,
				},
				UserID:    &memberUUID,
				UserEmail: "test@example.com",
			},
			expectedErr: "project id and name are required",
		},
		{
			name: "Project member removed with missing UserID",
			event: &events.ProjectMemberRemoved{
				ProjectBase: &events.ProjectBase{
					ProjectID:   &projectUUID,
					ProjectName: "test-project",
				},
				UserEmail: "test@example.com",
			},
			expectedErr: "user ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.event.ActionInfo()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}
