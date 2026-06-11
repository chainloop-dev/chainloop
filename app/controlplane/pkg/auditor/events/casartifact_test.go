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

func TestCASArtifactEvents(t *testing.T) {
	orgUUID, err := uuid.Parse("1089bb36-e27b-428b-8009-d015c8737c54")
	require.NoError(t, err)

	const (
		digest      = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
		fileName    = "sbom.cyclonedx.json"
		backendType = "OCI"
	)

	tests := []struct {
		name     string
		event    auditor.LogEntry
		expected string
	}{
		{
			name: "artifact uploaded",
			event: &events.CASArtifactUploaded{
				CASArtifactBase: &events.CASArtifactBase{
					Digest:      digest,
					SizeBytes:   1024,
					FileName:    fileName,
					BackendType: backendType,
				},
			},
			expected: "testdata/casartifacts/casartifact_uploaded.json",
		},
		{
			name: "artifact upload skipped (deduplicated)",
			event: &events.CASArtifactUploaded{
				CASArtifactBase: &events.CASArtifactBase{
					Digest:      digest,
					SizeBytes:   1024,
					FileName:    fileName,
					BackendType: backendType,
				},
				Skipped: true,
			},
			expected: "testdata/casartifacts/casartifact_upload_skipped.json",
		},
		{
			name: "artifact upload skipped with unknown size",
			event: &events.CASArtifactUploaded{
				CASArtifactBase: &events.CASArtifactBase{
					Digest:      digest,
					BackendType: backendType,
				},
				Skipped: true,
			},
			expected: "testdata/casartifacts/casartifact_upload_skipped_unknown_size.json",
		},
		{
			name: "artifact downloaded",
			event: &events.CASArtifactDownloaded{
				CASArtifactBase: &events.CASArtifactBase{
					Digest:      digest,
					SizeBytes:   1024,
					FileName:    fileName,
					BackendType: backendType,
				},
			},
			expected: "testdata/casartifacts/casartifact_downloaded.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// CAS artifact events are system-generated, no actor identity available
			require.False(t, tt.event.RequiresActor())
			require.Nil(t, tt.event.TargetID())
			require.Equal(t, events.CASArtifactType, tt.event.TargetType())

			eventPayload, err := auditor.GenerateAuditEvent(tt.event,
				auditor.WithOrgID(orgUUID),
				auditor.WithActor(auditor.ActorTypeSystem, uuid.Nil, "", ""),
			)
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

// TestCASArtifactEventsFailed tests the behavior of CAS artifact events when they are expected to fail
func TestCASArtifactEventsFailed(t *testing.T) {
	tests := []struct {
		name        string
		event       auditor.LogEntry
		expectedErr string
	}{
		{
			name: "artifact uploaded with missing digest",
			event: &events.CASArtifactUploaded{
				CASArtifactBase: &events.CASArtifactBase{
					SizeBytes: 1024,
					FileName:  "sbom.cyclonedx.json",
				},
			},
			expectedErr: "digest is required",
		},
		{
			name: "artifact downloaded with missing digest",
			event: &events.CASArtifactDownloaded{
				CASArtifactBase: &events.CASArtifactBase{
					SizeBytes: 1024,
				},
			},
			expectedErr: "digest is required",
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
