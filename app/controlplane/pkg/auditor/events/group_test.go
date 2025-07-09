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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupEvents(t *testing.T) {
	userUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	orgUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	groupUUID, err := uuid.Parse("3089bb36-e27b-428b-8009-d015c8737c56")
	require.NoError(t, err)
	memberUUID, err := uuid.Parse("4089bb36-e27b-428b-8009-d015c8737c57")
	require.NoError(t, err)
	groupName := "test-group"
	groupDescription := "test description"
	userEmail := "test@example.com"

	tests := []struct {
		name     string
		event    auditor.LogEntry
		expected string
		actor    auditor.ActorType
		actorID  uuid.UUID
	}{
		{
			name: "Group created by user",
			event: &events.GroupCreated{
				GroupBase: &events.GroupBase{
					GroupID:   &groupUUID,
					GroupName: groupName,
				},
				GroupDescription: groupDescription,
			},
			expected: "testdata/groups/group_created.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Group updated by user",
			event: &events.GroupUpdated{
				GroupBase: &events.GroupBase{
					GroupID:   &groupUUID,
					GroupName: groupName,
				},
				NewDescription: &groupDescription,
			},
			expected: "testdata/groups/group_updated.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Group updated with description by user",
			event: &events.GroupUpdated{
				GroupBase: &events.GroupBase{
					GroupID:   &groupUUID,
					GroupName: groupName,
				},
				NewDescription: &groupDescription,
			},
			expected: "testdata/groups/group_updated_with_description.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Group deleted by user",
			event: &events.GroupDeleted{
				GroupBase: &events.GroupBase{
					GroupID:   &groupUUID,
					GroupName: groupName,
				},
			},
			expected: "testdata/groups/group_deleted.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Group member added by user",
			event: &events.GroupMemberAdded{
				GroupBase: &events.GroupBase{
					GroupID:   &groupUUID,
					GroupName: groupName,
				},
				UserID:     &memberUUID,
				UserEmail:  userEmail,
				Maintainer: true,
			},
			expected: "testdata/groups/group_member_added.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Group member removed by user",
			event: &events.GroupMemberRemoved{
				GroupBase: &events.GroupBase{
					GroupID:   &groupUUID,
					GroupName: groupName,
				},
				UserID:    &memberUUID,
				UserEmail: userEmail,
			},
			expected: "testdata/groups/group_member_removed.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "Group member maintainer status updated by user",
			event: &events.GroupMemberUpdated{
				GroupBase: &events.GroupBase{
					GroupID:   &groupUUID,
					GroupName: groupName,
				},
				UserID:              &memberUUID,
				UserEmail:           userEmail,
				NewMaintainerStatus: true,
			},
			expected: "testdata/groups/group_member_updated.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []auditor.GeneratorOption{
				auditor.WithOrgID(orgUUID),
			}

			if tt.actor == auditor.ActorTypeUser {
				opts = append(opts, auditor.WithActor(auditor.ActorTypeUser, tt.actorID, testEmail))
			} else {
				opts = append(opts, auditor.WithActor(auditor.ActorTypeSystem, uuid.Nil, ""))
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

// TestGroupEventsFailed tests the behavior of group events when they are expected to fail
func TestGroupEventsFailed(t *testing.T) {
	groupUUID, err := uuid.Parse("3089bb36-e27b-428b-8009-d015c8737c56")
	require.NoError(t, err)
	memberUUID, err := uuid.Parse("4089bb36-e27b-428b-8009-d015c8737c57")
	require.NoError(t, err)
	groupDescription := "test description"

	tests := []struct {
		name        string
		event       auditor.LogEntry
		expectedErr string
	}{
		{
			name: "Group created with missing GroupID",
			event: &events.GroupCreated{
				GroupBase: &events.GroupBase{
					GroupName: "test-group",
				},
				GroupDescription: groupDescription,
			},
			expectedErr: "group id and name are required",
		},
		{
			name: "Group created with missing GroupName",
			event: &events.GroupCreated{
				GroupBase: &events.GroupBase{
					GroupID: &groupUUID,
				},
				GroupDescription: groupDescription,
			},
			expectedErr: "group id and name are required",
		},
		{
			name: "Group updated with missing GroupID",
			event: &events.GroupUpdated{
				GroupBase: &events.GroupBase{
					GroupName: "test-group",
				},
				NewDescription: &groupDescription,
			},
			expectedErr: "group id and name are required",
		},
		{
			name: "Group updated with missing GroupName",
			event: &events.GroupUpdated{
				GroupBase: &events.GroupBase{
					GroupID: &groupUUID,
				},
				NewDescription: &groupDescription,
			},
			expectedErr: "group id and name are required",
		},
		{
			name: "Group deleted with missing GroupID",
			event: &events.GroupDeleted{
				GroupBase: &events.GroupBase{
					GroupName: "test-group",
				},
			},
			expectedErr: "group id and name are required",
		},
		{
			name: "Group deleted with missing GroupName",
			event: &events.GroupDeleted{
				GroupBase: &events.GroupBase{
					GroupID: &groupUUID,
				},
			},
			expectedErr: "group id and name are required",
		},
		{
			name: "Group member added with missing GroupID",
			event: &events.GroupMemberAdded{
				GroupBase: &events.GroupBase{
					GroupName: "test-group",
				},
				UserID:     &memberUUID,
				UserEmail:  "test@example.com",
				Maintainer: true,
			},
			expectedErr: "group id and name are required",
		},
		{
			name: "Group member added with missing GroupName",
			event: &events.GroupMemberAdded{
				GroupBase: &events.GroupBase{
					GroupID: &groupUUID,
				},
				UserID:     &memberUUID,
				UserEmail:  "test@example.com",
				Maintainer: true,
			},
			expectedErr: "group id and name are required",
		},
		{
			name: "Group member added with missing UserID",
			event: &events.GroupMemberAdded{
				GroupBase: &events.GroupBase{
					GroupID:   &groupUUID,
					GroupName: "test-group",
				},
				UserEmail:  "test@example.com",
				Maintainer: true,
			},
			expectedErr: "user ID is required",
		},
		{
			name: "Group member removed with missing GroupID",
			event: &events.GroupMemberRemoved{
				GroupBase: &events.GroupBase{
					GroupName: "test-group",
				},
				UserID:    &memberUUID,
				UserEmail: "test@example.com",
			},
			expectedErr: "group id and name are required",
		},
		{
			name: "Group member removed with missing GroupName",
			event: &events.GroupMemberRemoved{
				GroupBase: &events.GroupBase{
					GroupID: &groupUUID,
				},
				UserID:    &memberUUID,
				UserEmail: "test@example.com",
			},
			expectedErr: "group id and name are required",
		},
		{
			name: "Group member removed with missing UserID",
			event: &events.GroupMemberRemoved{
				GroupBase: &events.GroupBase{
					GroupID:   &groupUUID,
					GroupName: "test-group",
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
