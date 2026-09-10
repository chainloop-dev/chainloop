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
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bannerProvider is a real provider with its banner capability forced, so a
// test can drive both sides of the gate without a second agent.
type bannerProvider struct {
	trace.Provider

	supports bool
	sysCalls int
}

func (p *bannerProvider) SupportsSystemMessage() bool { return p.supports }

func (p *bannerProvider) SystemMessage(string) error {
	p.sysCalls++

	return nil
}

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
			dashboardURL: testDashboardURL,
			want:         recording + "\nEvidence will be sent to " + testDashboardURL,
		},
		{
			name:         "trailing slash is trimmed",
			dashboardURL: testDashboardURL + "/",
			want:         recording + "\nEvidence will be sent to " + testDashboardURL,
		},
		{
			// Destination and identity share one line, and the URL keeps a
			// space after it so a terminal linkifies it without swallowing
			// the parenthesis.
			name:         "organization and project qualify the destination",
			dashboardURL: testDashboardURL,
			org:          testOrgName,
			project:      testProject,
			want: recording +
				"\nEvidence will be sent to " + testDashboardURL +
				" (organization: " + testOrgName + ", project: " + testProject + ")",
		},
		{
			name:    "identity stands alone when no dashboard is configured",
			org:     testOrgName,
			project: testProject,
			want:    recording + "\norganization: " + testOrgName + ", project: " + testProject,
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
		{
			name:         "dashboard with only a project",
			dashboardURL: testDashboardURL,
			project:      testProject,
			want: recording +
				"\nEvidence will be sent to " + testDashboardURL +
				" (project: " + testProject + ")",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sessionStartBanner(tc.dashboardURL, tc.org, tc.project))
		})
	}
}

// TestSessionStartBannerGate checks that an agent which cannot display the
// banner is never handed one. Building it costs a control-plane round trip,
// which is not worth paying for a string the agent throws away.
func TestSessionStartBannerGate(t *testing.T) {
	testCases := []struct {
		name     string
		supports bool
		wantSent int
	}{
		{
			name:     "an agent that shows messages gets the banner",
			supports: true,
			wantSent: 1,
		},
		{
			name:     "an agent that discards them is not asked",
			supports: false,
			wantSent: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := initTempGitRepo(t)
			store := state.NewGitStore(filepath.Join(repoDir, ".git"))
			require.NoError(t, store.InitTraceDir())

			origDir, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(repoDir))
			t.Cleanup(func() { _ = os.Chdir(origDir) })

			withStdin(t, `{"session_id":"abc-123"}`)

			p := &bannerProvider{Provider: claude.New(), supports: tc.supports}
			require.NoError(t, HandleAgentSessionStart(p, zerolog.Nop()))

			assert.Equal(t, tc.wantSent, p.sysCalls)
			assert.True(t, store.SessionRecordExists("abc-123"), "the session is tracked either way")
		})
	}
}
