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
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateGolden bool

func TestMain(m *testing.M) {
	flag.BoolVar(&updateGolden, "update-golden", false, "update the expected golden files")
	// Parse the flags
	flag.Parse()
	os.Exit(m.Run())
}

func TestUserEvents(t *testing.T) {
	userUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	orgUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	const testEmail = "john@cyberdyne.io"

	tests := []struct {
		name     string
		event    auditor.LogEntry
		expected string
	}{
		{
			name: "User signs up",
			event: &events.UserSignedUp{
				UserBase: &events.UserBase{
					UserID: uuidPtr(userUUID),
					Email:  testEmail,
				},
			},
			expected: "testdata/user_signs_up.json",
		},
		{
			name: "User logs in",
			event: &events.UserLoggedIn{
				UserBase: &events.UserBase{
					UserID: uuidPtr(userUUID),
					Email:  testEmail,
				},
				LoggedIn: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
			},
			expected: "testdata/user_logs_in.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []auditor.GeneratorOption{
				auditor.WithActor(auditor.ActorTypeUser, userUUID, testEmail),
				auditor.WithOrgID(orgUUID),
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

func uuidPtr(id uuid.UUID) *uuid.UUID {
	return &id
}
