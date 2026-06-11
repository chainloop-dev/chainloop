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

package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePublisher records published payloads and optionally fails
type fakePublisher struct {
	published []*auditor.EventPayload
	err       error
}

func (f *fakePublisher) Publish(data *auditor.EventPayload) error {
	if f.err != nil {
		return f.err
	}

	f.published = append(f.published, data)
	return nil
}

func newTestDispatcher(p eventPublisher) *AuditDispatcher {
	return &AuditDispatcher{publisher: p, log: servicelogger.ScopedHelper(log.DefaultLogger, "test")}
}

func testUploadedEntry() auditor.LogEntry {
	return &events.CASArtifactUploaded{
		CASArtifactBase: &events.CASArtifactBase{
			Digest:      "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
			SizeBytes:   11,
			FileName:    "test.txt",
			BackendType: "OCI",
		},
	}
}

const testOrgID = "1089bb36-e27b-428b-8009-d015c8737c54"

func TestAuditDispatcherDispatch(t *testing.T) {
	tests := []struct {
		name          string
		dispatcher    *AuditDispatcher
		entry         auditor.LogEntry
		claims        *casJWT.Claims
		wantPublished int
	}{
		{
			name:       "nil dispatcher is a no-op",
			dispatcher: nil,
			entry:      testUploadedEntry(),
			claims:     &casJWT.Claims{OrgID: testOrgID},
		},
		{
			name:       "nil publisher is a no-op",
			dispatcher: newTestDispatcher(nil),
			entry:      testUploadedEntry(),
			claims:     &casJWT.Claims{OrgID: testOrgID},
		},
		{
			name:       "internal control plane traffic is skipped",
			dispatcher: newTestDispatcher(&fakePublisher{}),
			entry:      testUploadedEntry(),
			claims:     &casJWT.Claims{OrgID: testOrgID, SourceInternal: true},
		},
		{
			name:       "invalid org id is skipped",
			dispatcher: newTestDispatcher(&fakePublisher{}),
			entry:      testUploadedEntry(),
			claims:     &casJWT.Claims{OrgID: "not-an-uuid"},
		},
		{
			name:       "invalid entry is skipped",
			dispatcher: newTestDispatcher(&fakePublisher{}),
			entry:      &events.CASArtifactUploaded{CASArtifactBase: &events.CASArtifactBase{}},
			claims:     &casJWT.Claims{OrgID: testOrgID},
		},
		{
			name:       "publish errors are swallowed",
			dispatcher: newTestDispatcher(&fakePublisher{err: errors.New("nats is down")}),
			entry:      testUploadedEntry(),
			claims:     &casJWT.Claims{OrgID: testOrgID},
		},
		{
			name:          "client traffic is published",
			dispatcher:    newTestDispatcher(&fakePublisher{}),
			entry:         testUploadedEntry(),
			claims:        &casJWT.Claims{OrgID: testOrgID},
			wantPublished: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// must never panic nor return an error
			tc.dispatcher.Dispatch(tc.entry, tc.claims)

			if tc.dispatcher == nil || tc.dispatcher.publisher == nil {
				return
			}
			fake := tc.dispatcher.publisher.(*fakePublisher)

			require.Len(t, fake.published, tc.wantPublished)
			if tc.wantPublished == 0 {
				return
			}

			got := fake.published[0]
			assert.Equal(t, auditor.AuditEventType, got.EventType)
			assert.Equal(t, events.CASArtifactUploadedActionType, got.Data.ActionType)
			assert.Equal(t, events.CASArtifactType, got.Data.TargetType)
			assert.Equal(t, auditor.ActorType(auditor.ActorTypeSystem), got.Data.ActorType)
			require.NotNil(t, got.Data.OrgID)
			assert.Equal(t, testOrgID, got.Data.OrgID.String())
		})
	}
}

func TestAuditDispatcherShouldEmit(t *testing.T) {
	tests := []struct {
		name       string
		dispatcher *AuditDispatcher
		claims     *casJWT.Claims
		want       bool
	}{
		{name: "nil dispatcher", dispatcher: nil, claims: &casJWT.Claims{}, want: false},
		{name: "nil publisher", dispatcher: newTestDispatcher(nil), claims: &casJWT.Claims{}, want: false},
		{name: "nil claims", dispatcher: newTestDispatcher(&fakePublisher{}), claims: nil, want: false},
		{name: "internal traffic", dispatcher: newTestDispatcher(&fakePublisher{}), claims: &casJWT.Claims{SourceInternal: true}, want: false},
		{name: "client traffic", dispatcher: newTestDispatcher(&fakePublisher{}), claims: &casJWT.Claims{}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.dispatcher.shouldEmit(tc.claims))
		})
	}
}

// artifactEventInfo mirrors the action info payload of CAS artifact events for assertions
type artifactEventInfo struct {
	Digest      string `json:"digest"`
	SizeBytes   int64  `json:"size_bytes"`
	FileName    string `json:"file_name"`
	BackendType string `json:"backend_type"`
	Skipped     bool   `json:"skipped"`
}

func decodeArtifactEvent(t *testing.T, payload *auditor.EventPayload) *artifactEventInfo {
	t.Helper()

	info := &artifactEventInfo{}
	require.NoError(t, json.Unmarshal(payload.Data.Info, info))
	return info
}
