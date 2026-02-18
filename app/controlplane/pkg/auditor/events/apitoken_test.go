//
// Copyright 2025-2026 The Chainloop Authors.
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
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPITokenEvents(t *testing.T) {
	userUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	apiTokenUUID, err := uuid.Parse("2089bb36-e27b-428b-8009-d015c8737c55")
	require.NoError(t, err)
	orgUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)
	apiTokenName := "test-token"
	apiTokenDescription := "test description"
	expirationDate, err := time.Parse(time.RFC3339, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	tests := []struct {
		name     string
		event    auditor.LogEntry
		expected string
		actor    auditor.ActorType
		actorID  uuid.UUID
	}{
		{
			name: "API Token created by user",
			event: &events.APITokenCreated{
				APITokenBase: &events.APITokenBase{
					APITokenID:   uuidPtr(apiTokenUUID),
					APITokenName: apiTokenName,
				},
			},
			expected: "testdata/apitokens/api_token_created.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "API Token created with description by user",
			event: &events.APITokenCreated{
				APITokenBase: &events.APITokenBase{
					APITokenID:   uuidPtr(apiTokenUUID),
					APITokenName: apiTokenName,
				},
				APITokenDescription: &apiTokenDescription,
			},
			expected: "testdata/apitokens/api_token_created_with_description.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "API Token created with expires at by user",
			event: &events.APITokenCreated{
				APITokenBase: &events.APITokenBase{
					APITokenID:   uuidPtr(apiTokenUUID),
					APITokenName: apiTokenName,
				},
				APITokenDescription: &apiTokenDescription,
				ExpiresAt:           &expirationDate,
			},
			expected: "testdata/apitokens/api_token_created_with_expiration_date.json",
			actor:    auditor.ActorTypeUser,
			actorID:  userUUID,
		},
		{
			name: "API Token revoked by user",
			event: &events.APITokenRevoked{
				APITokenBase: &events.APITokenBase{
					APITokenID:   uuidPtr(apiTokenUUID),
					APITokenName: apiTokenName,
				},
			},
			expected: "testdata/apitokens/api_token_revoked.json",
			actor:    auditor.ActorTypeAPIToken,
			actorID:  apiTokenUUID,
		},
		{
			name: "API Token auto-revoked by system",
			event: &events.APITokenRevoked{
				APITokenBase: &events.APITokenBase{
					APITokenID:   uuidPtr(apiTokenUUID),
					APITokenName: apiTokenName,
				},
			},
			expected: "testdata/apitokens/api_token_revoked_by_system.json",
			actor:    auditor.ActorTypeSystem,
			actorID:  uuid.Nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []auditor.GeneratorOption{
				auditor.WithOrgID(orgUUID),
			}
			switch tt.actor {
			case auditor.ActorTypeAPIToken:
				opts = append(opts, auditor.WithActor(auditor.ActorTypeAPIToken, tt.actorID, "", testAPITokenName))
			case auditor.ActorTypeSystem:
				opts = append(opts, auditor.WithActor(auditor.ActorTypeSystem, uuid.Nil, "", ""))
			default:
				opts = append(opts, auditor.WithActor(auditor.ActorTypeUser, tt.actorID, testEmail, testName))
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
