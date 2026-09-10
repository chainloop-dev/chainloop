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

// TestSessionStartBanner pins what the developer is told at the top of a
// session. The audience includes a teammate who cloned the repository and
// never ran init, so the banner has to say what is happening and where the
// evidence goes without assuming any Chainloop vocabulary.
func TestSessionStartBanner(t *testing.T) {
	const (
		recording   = "Chainloop Trace is recording this session."
		testProject = "chainloop-cli"
	)

	testCases := []struct {
		name         string
		dashboardURL string
		org          string
		project      string
		want         string
	}{
		{
			name: "nothing known says only what is happening",
			want: recording,
		},
		{
			name:         "destination is named when there is a dashboard",
			dashboardURL: "https://app.chainloop.dev",
			want:         recording + "\nEvidence will be sent to https://app.chainloop.dev",
		},
		{
			name:         "trailing slash is trimmed",
			dashboardURL: "https://app.chainloop.dev/",
			want:         recording + "\nEvidence will be sent to https://app.chainloop.dev",
		},
		{
			name:         "organization and project are named when known",
			dashboardURL: "https://app.chainloop.dev",
			org:          testOrgName,
			project:      testProject,
			want: recording +
				"\nEvidence will be sent to https://app.chainloop.dev" +
				"\norganization: " + testOrgName + "  project: " + testProject,
		},
		{
			name:    "identity is named even with no dashboard configured",
			org:     testOrgName,
			project: testProject,
			want:    recording + "\norganization: " + testOrgName + "  project: " + testProject,
		},
		{
			name:    "project alone",
			project: testProject,
			want:    recording + "\nproject: " + testProject,
		},
		{
			name: "organization alone",
			org:  testOrgName,
			want: recording + "\norganization: " + testOrgName,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sessionStartBanner(tc.dashboardURL, tc.org, tc.project))
		})
	}
}
