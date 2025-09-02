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

func TestCASBackendEvents(t *testing.T) {
	userUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	orgUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	backendUUID, err := uuid.Parse("3089bb36-e27b-428b-8009-d015c8737c56")
	require.NoError(t, err)

	backendName := "test-backend"
	backendDescription := "test description"
	backendLocation := "test-location"
	backendProvider := "OCI"

	tests := []struct {
		name     string
		event    auditor.LogEntry
		expected string
		actor    auditor.ActorType
		actorID  uuid.UUID
	}{
		{
			name: "CAS Backend created by user",
			event: &events.CASBackendCreated{
				CASBackendBase: &events.CASBackendBase{
					CASBackendID:   &backendUUID,
					CASBackendName: backendName,
					Provider:       backendProvider,
					Location:       backendLocation,
					Default:        true,
				},
				CASBackendDescription: backendDescription,
			},
			expected: "testdata/casbackends/casbackend_created.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "CAS Backend updated by user",
			event: &events.CASBackendUpdated{
				CASBackendBase: &events.CASBackendBase{
					CASBackendID:   &backendUUID,
					CASBackendName: backendName,
					Provider:       backendProvider,
					Location:       backendLocation,
					Default:        true,
				},
				NewDescription:     &backendDescription,
				CredentialsChanged: true,
				PreviousDefault:    false,
			},
			expected: "testdata/casbackends/casbackend_updated.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "CAS Backend updated by user without credential change",
			event: &events.CASBackendUpdated{
				CASBackendBase: &events.CASBackendBase{
					CASBackendID:   &backendUUID,
					CASBackendName: backendName,
					Provider:       backendProvider,
					Location:       backendLocation,
					Default:        false,
				},
				NewDescription:     &backendDescription,
				CredentialsChanged: false,
				PreviousDefault:    true,
			},
			expected: "testdata/casbackends/casbackend_updated_default_change.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "CAS Backend deleted by user",
			event: &events.CASBackendDeleted{
				CASBackendBase: &events.CASBackendBase{
					CASBackendID:   &backendUUID,
					CASBackendName: backendName,
					Provider:       backendProvider,
					Location:       backendLocation,
					Default:        true,
				},
			},
			expected: "testdata/casbackends/casbackend_deleted.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "CAS Backend status changed",
			event: &events.CASBackendStatusChanged{
				CASBackendBase: &events.CASBackendBase{
					CASBackendID:   &backendUUID,
					CASBackendName: backendName,
					Provider:       backendProvider,
					Location:       backendLocation,
					Default:        true,
				},
				PreviousStatus: "Invalid",
				NewStatus:      "OK",
				IsRecovery:     true,
			},
			expected: "testdata/casbackends/casbackend_status_recovery.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "CAS Backend status changed without recovery",
			event: &events.CASBackendStatusChanged{
				CASBackendBase: &events.CASBackendBase{
					CASBackendID:   &backendUUID,
					CASBackendName: backendName,
					Provider:       backendProvider,
					Location:       backendLocation,
					Default:        true,
				},
				PreviousStatus: "OK",
				NewStatus:      "Invalid",
				IsRecovery:     false,
			},
			expected: "testdata/casbackends/casbackend_status_change.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "CAS Backend created by system",
			event: &events.CASBackendCreated{
				CASBackendBase: &events.CASBackendBase{
					CASBackendID:   &backendUUID,
					CASBackendName: backendName,
					Provider:       backendProvider,
					Location:       backendLocation,
					Default:        true,
				},
				CASBackendDescription: backendDescription,
			},
			expected: "testdata/casbackends/casbackend_created_by_system.json",
			actor:    auditor.ActorTypeSystem,
			actorID:  uuid.Nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []auditor.GeneratorOption{
				auditor.WithOrgID(orgUUID),
			}

			if tt.actor == auditor.ActorTypeUser {
				opts = append(opts, auditor.WithActor(auditor.ActorTypeUser, tt.actorID, testEmail, testName))
			} else {
				opts = append(opts, auditor.WithActor(auditor.ActorTypeSystem, uuid.Nil, "", testAPITokenName))
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

// TestCASBackendEventsFailed tests the behavior of CAS backend events when they are expected to fail
func TestCASBackendEventsFailed(t *testing.T) {
	backendUUID, err := uuid.Parse("3089bb36-e27b-428b-8009-d015c8737c56")
	require.NoError(t, err)
	backendDescription := "test description"

	tests := []struct {
		name        string
		event       auditor.LogEntry
		expectedErr string
	}{
		{
			name: "CAS Backend created with missing ID",
			event: &events.CASBackendCreated{
				CASBackendBase: &events.CASBackendBase{
					CASBackendName: "test-backend",
					Provider:       "OCI",
					Location:       "test-location",
				},
				CASBackendDescription: backendDescription,
			},
			expectedErr: "cas backend id and name are required",
		},
		{
			name: "CAS Backend created with missing name",
			event: &events.CASBackendCreated{
				CASBackendBase: &events.CASBackendBase{
					CASBackendID: &backendUUID,
					Provider:     "OCI",
					Location:     "test-location",
				},
				CASBackendDescription: backendDescription,
			},
			expectedErr: "cas backend id and name are required",
		},
		{
			name: "CAS Backend updated with missing ID",
			event: &events.CASBackendUpdated{
				CASBackendBase: &events.CASBackendBase{
					CASBackendName: "test-backend",
					Provider:       "OCI",
					Location:       "test-location",
				},
				NewDescription:     &backendDescription,
				CredentialsChanged: true,
			},
			expectedErr: "cas backend id and name are required",
		},
		{
			name: "CAS Backend updated with missing name",
			event: &events.CASBackendUpdated{
				CASBackendBase: &events.CASBackendBase{
					CASBackendID: &backendUUID,
					Provider:     "OCI",
					Location:     "test-location",
				},
				NewDescription:     &backendDescription,
				CredentialsChanged: true,
			},
			expectedErr: "cas backend id and name are required",
		},
		{
			name: "CAS Backend deleted with missing ID",
			event: &events.CASBackendDeleted{
				CASBackendBase: &events.CASBackendBase{
					CASBackendName: "test-backend",
					Provider:       "OCI",
					Location:       "test-location",
				},
			},
			expectedErr: "cas backend id and name are required",
		},
		{
			name: "CAS Backend deleted with missing name",
			event: &events.CASBackendDeleted{
				CASBackendBase: &events.CASBackendBase{
					CASBackendID: &backendUUID,
					Provider:     "OCI",
					Location:     "test-location",
				},
			},
			expectedErr: "cas backend id and name are required",
		},
		{
			name: "CAS Backend status changed with missing ID",
			event: &events.CASBackendStatusChanged{
				CASBackendBase: &events.CASBackendBase{
					CASBackendName: "test-backend",
					Provider:       "OCI",
					Location:       "test-location",
				},
				PreviousStatus: "Invalid",
				NewStatus:      "OK",
			},
			expectedErr: "cas backend id and name are required",
		},
		{
			name: "CAS Backend status changed with missing name",
			event: &events.CASBackendStatusChanged{
				CASBackendBase: &events.CASBackendBase{
					CASBackendID: &backendUUID,
					Provider:     "OCI",
					Location:     "test-location",
				},
				PreviousStatus: "Invalid",
				NewStatus:      "OK",
			},
			expectedErr: "cas backend id and name are required",
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
