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

func TestOrganizationEvents(t *testing.T) {
	actorUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	orgUUID, err := uuid.Parse("2089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	removedUserUUID, err := uuid.Parse("3089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)

	tests := []struct {
		name     string
		event    auditor.LogEntry
		expected string
	}{
		{
			name: "User removed by admin",
			event: &events.OrgUserRemoved{
				OrgBase: &events.OrgBase{
					OrgID:   uuidPtr(orgUUID),
					OrgName: "cyberdyne",
				},
				RemovedUserID:    removedUserUUID,
				RemovedUserEmail: "sarah@cyberdyne.io",
			},
			expected: "testdata/organization/user_removed.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []auditor.GeneratorOption{
				auditor.WithActor(auditor.ActorTypeUser, actorUUID, testEmail, testName),
				auditor.WithOrgID(orgUUID),
			}

			eventPayload, err := auditor.GenerateAuditEvent(tt.event, opts...)
			require.NoError(t, err)

			want, err := json.MarshalIndent(eventPayload.Data, "", "  ")
			require.NoError(t, err)

			if updateGolden {
				err := os.MkdirAll(filepath.Dir(tt.expected), 0750)
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
