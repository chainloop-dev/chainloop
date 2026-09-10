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

package action

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Shared across the action package's tests.
const (
	testDashboardURL = "https://app.chainloop.dev"
	testOrgName      = "chainloop"
	testSessionID    = "ses_1"
)

func TestBuildSessionViewURL(t *testing.T) {
	const uuidSessionID = "1d75645b-fd6b-4e7c-9855-f3137add4cf7"

	testCases := []struct {
		name           string
		uiDashboardURL string
		orgName        string
		sessionID      string
		want           string
	}{
		{
			name:           "no dashboard configured",
			uiDashboardURL: "",
			orgName:        testOrgName,
			sessionID:      uuidSessionID,
			want:           "",
		},
		{
			name:           "empty session id",
			uiDashboardURL: testDashboardURL,
			orgName:        testOrgName,
			sessionID:      "",
			want:           "",
		},
		{
			name:           "uuid session id",
			uiDashboardURL: testDashboardURL,
			orgName:        testOrgName,
			sessionID:      uuidSessionID,
			want:           testDashboardURL + "/u/chainloop/sessions/" + uuidSessionID,
		},
		{
			name:           "trailing slash is trimmed",
			uiDashboardURL: testDashboardURL + "/",
			orgName:        testOrgName,
			sessionID:      uuidSessionID,
			want:           testDashboardURL + "/u/chainloop/sessions/" + uuidSessionID,
		},
		{
			// OpenCode session IDs are not UUIDs; kept as a guard that a
			// non-UUID agent ID still passes through unchanged.
			name:           "opencode style session id",
			uiDashboardURL: testDashboardURL,
			orgName:        testOrgName,
			sessionID:      "ses_8ab12cd34",
			want:           testDashboardURL + "/u/chainloop/sessions/ses_8ab12cd34",
		},
		{
			name:           "session id is path escaped",
			uiDashboardURL: testDashboardURL,
			orgName:        testOrgName,
			sessionID:      "a b/c",
			want:           testDashboardURL + "/u/chainloop/sessions/a%20b%2Fc",
		},
		{
			name:           "org name is path escaped",
			uiDashboardURL: testDashboardURL,
			orgName:        "my org",
			sessionID:      testSessionID,
			want:           testDashboardURL + "/u/my%20org/sessions/" + testSessionID,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildSessionViewURL(tc.uiDashboardURL, tc.orgName, tc.sessionID)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestAttestationResultGetOrganization covers the accessor RunTracePush feeds
// into the session link, including the nil links in the Status chain.
func TestAttestationResultGetOrganization(t *testing.T) {
	testCases := []struct {
		name string
		res  *AttestationResult
		want string
	}{
		{name: "nil result", res: nil},
		{name: "nil status", res: &AttestationResult{}},
		{name: "nil workflow meta", res: &AttestationResult{Status: &AttestationStatusResult{}}},
		{
			name: "organization reported by the control plane",
			res: &AttestationResult{Status: &AttestationStatusResult{
				WorkflowMeta: &AttestationStatusWorkflowMeta{Organization: testOrgName},
			}},
			want: testOrgName,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.res.GetOrganization())
		})
	}
}

// TestBuildAttestationViewURL guards the route the session link now shares a
// builder with, so the refactor cannot silently change it.
func TestBuildAttestationViewURL(t *testing.T) {
	const digest = "sha256:abc123"

	want := testDashboardURL + "/u/chainloop/workflow-runs/" + digest
	assert.Equal(t, want, buildAttestationViewURL(testDashboardURL, testOrgName, digest))
	assert.Empty(t, buildAttestationViewURL("", testOrgName, digest))
	assert.Empty(t, buildAttestationViewURL(testDashboardURL, testOrgName, ""))
}
