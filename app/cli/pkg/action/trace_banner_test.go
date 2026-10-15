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
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// bannerProvider is a real provider with its session-start capability forced,
// so a test can drive both sides of the gate without a second agent. It also
// captures what it was handed, keeping the message off the test's stdout.
type bannerProvider struct {
	trace.Provider

	banner      bool
	instruction bool
	sysCalls    int
	last        trace.SessionStartMessage
}

func (p *bannerProvider) SupportsSessionStartBanner() bool      { return p.banner }
func (p *bannerProvider) SupportsSessionStartInstruction() bool { return p.instruction }

func (p *bannerProvider) AnnounceSessionStart(msg trace.SessionStartMessage) error {
	p.sysCalls++
	p.last = msg

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

// TestSessionStartChannelGate checks that each part of the session-start
// message is composed only for an agent that can receive it. Building the
// banner costs a control-plane round trip, which is not worth paying for a
// string the agent throws away, and an agent with no model channel has no use
// for the instruction.
func TestSessionStartChannelGate(t *testing.T) {
	testCases := []struct {
		name            string
		banner          bool
		instruction     bool
		wantSent        int
		wantBanner      bool
		wantInstruction bool
	}{
		{
			name:   "an agent with both channels gets both",
			banner: true, instruction: true,
			wantSent: 1, wantBanner: true, wantInstruction: true,
		},
		{
			// Cursor and opencode: the model hears the instruction, and
			// nobody pays for a banner.
			name:        "an agent with only a model channel gets only the instruction",
			instruction: true,
			wantSent:    1, wantInstruction: true,
		},
		{
			name:     "an agent with only a user channel gets only the banner",
			banner:   true,
			wantSent: 1, wantBanner: true,
		},
		{
			name:     "an agent with neither is not asked",
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

			p := &bannerProvider{Provider: claude.New(), banner: tc.banner, instruction: tc.instruction}
			require.NoError(t, HandleAgentSessionStart(p, nil, zerolog.Nop()))

			assert.Equal(t, tc.wantSent, p.sysCalls)
			if tc.wantBanner {
				assert.Contains(t, p.last.Banner, "Chainloop Trace is recording this session.")
			} else {
				assert.Empty(t, p.last.Banner)
			}
			if tc.wantInstruction {
				assert.Contains(t, p.last.Instruction, "Write the specification")
			} else {
				assert.Empty(t, p.last.Instruction)
			}
			assert.True(t, store.SessionRecordExists("abc-123"), "the session is tracked either way")
		})
	}
}

// TestSessionStartAuthProblem checks that credentials that the control plane
// does not accept are reported at the start of the session. Otherwise the
// developer learns about it only when the push does not upload the session.
func TestSessionStartAuthProblem(t *testing.T) {
	expired := func(context.Context, string) error {
		return status.Error(codes.Unauthenticated, jwtMiddleware.ErrTokenExpired.Message)
	}
	const wantMsg = `your authentication token has expired, please run "chainloop auth login" again`

	testCases := []struct {
		name        string
		checkAuth   TraceAuthCheck
		local       bool
		banner      bool
		wantWarning bool
	}{
		{name: "an auth failure is reported in the banner", checkAuth: expired, banner: true, wantWarning: true},
		{name: "an auth failure is reported in the instruction when there is no banner", checkAuth: expired, wantWarning: true},
		{name: "accepted credentials are not reported", checkAuth: func(context.Context, string) error { return nil }, banner: true},
		{name: "an unreachable control plane is not reported", checkAuth: func(context.Context, string) error {
			return status.Error(codes.Unavailable, "connection refused")
		}, banner: true},
		{name: "no check is not reported", banner: true},
		{name: "local mode does not check", checkAuth: func(context.Context, string) error {
			return status.Error(codes.Unauthenticated, jwtMiddleware.ErrTokenExpired.Message)
		}, local: true, banner: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := initTempGitRepo(t)
			store := state.NewGitStore(filepath.Join(repoDir, ".git"))
			require.NoError(t, store.InitTraceDir())
			if tc.local {
				require.NoError(t, config.SaveTraceStorageToYML(repoDir, config.TraceStorageLocal))
			}
			t.Chdir(repoDir)

			withStdin(t, `{"session_id":"abc-123"}`)

			p := &bannerProvider{Provider: claude.New(), banner: tc.banner, instruction: true}
			require.NoError(t, HandleAgentSessionStart(p, tc.checkAuth, zerolog.Nop()))

			if !tc.wantWarning {
				assert.NotContains(t, p.last.Banner, wantMsg)
				assert.NotContains(t, p.last.Instruction, wantMsg)
				return
			}

			if tc.banner {
				assert.Contains(t, p.last.Banner, wantMsg)
				assert.NotContains(t, p.last.Instruction, wantMsg, "the banner already tells the developer")
			} else {
				assert.Contains(t, p.last.Instruction, wantMsg)
			}
		})
	}
}

// TestSessionAuthProblemTimeout checks that a slow auth check does not hold
// the session start, and shows no warning.
func TestSessionAuthProblemTimeout(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	slow := func(context.Context, string) error {
		<-release
		return status.Error(codes.Unauthenticated, jwtMiddleware.ErrTokenExpired.Message)
	}

	start := time.Now()
	assert.Empty(t, sessionAuthProblem(slow, "", zerolog.Nop()))
	assert.Less(t, time.Since(start), hookAuthCheckTimeout+time.Second)
}
