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

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/token"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	testCmdWorkflow     = "workflow"
	testCmdList         = "list"
	testCmdWorkflowList = testCmdWorkflow + " " + testCmdList
	testOrgName         = "my-org"
	testCPURL           = "api.cp.chainloop.dev:443"
)

// newTestRootCommand builds a root with the same name the real one uses, so the helpers
// that walk up to it behave as they do in production.
func newTestRootCommand() *cobra.Command {
	return &cobra.Command{Use: appName}
}

// newReportableCommand returns a plain subcommand attached to a root: the ordinary case
// that telemetry does report.
func newReportableCommand() *cobra.Command {
	rootCmd := newTestRootCommand()
	child := &cobra.Command{Use: testCmdList}
	rootCmd.AddCommand(child)

	return child
}

func TestClassifyError(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "no error",
			err:  nil,
			want: "",
		},
		{
			name: "plain error",
			err:  errors.New("something went wrong"),
			want: errorKindOther,
		},
		{
			name: "grpc unavailable",
			err:  status.Error(codes.Unavailable, "control plane is down"),
			want: "Unavailable",
		},
		{
			name: "grpc permission denied",
			err:  status.Error(codes.PermissionDenied, "token is not allowed"),
			want: "PermissionDenied",
		},
		{
			name: "wrapped grpc not found",
			err:  fmt.Errorf("looking up workflow: %w", status.Error(codes.NotFound, "workflow not found")),
			want: "NotFound",
		},
		{
			name: "doubly wrapped grpc error",
			err:  fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", status.Error(codes.AlreadyExists, "duplicated"))),
			want: "AlreadyExists",
		},
		{
			name: "wrapped plain error",
			err:  fmt.Errorf("reading file: %w", errors.New("no such file")),
			want: errorKindOther,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, classifyError(tc.err))
		})
	}
}

// TestClassifyErrorNeverLeaksMessage pins the promise that only a class of error is
// reported. The message can carry file paths, organization names and material names.
func TestClassifyErrorNeverLeaksMessage(t *testing.T) {
	secret := "/home/someone/private/material.json"
	testCases := []error{
		errors.New(secret),
		fmt.Errorf("adding material: %w", errors.New(secret)),
		status.Error(codes.InvalidArgument, secret),
	}

	for _, err := range testCases {
		assert.NotContains(t, classifyError(err), secret)
	}
}

func TestBuildCompletedCommand(t *testing.T) {
	userIdentity := &token.ParsedToken{
		ID:        "6f2e4a1b-0000-4a4a-9a1e-8a6b4c2d1e0f",
		OrgID:     "3b8f3b1e-0b7a-4a4a-9a1e-8a6b4c2d1e0f",
		TokenType: v1.Attestation_Auth_AUTH_TYPE_USER,
	}

	testCases := []struct {
		name     string
		duration time.Duration
		runErr   error
		identity *token.ParsedToken
		orgName  string
		want     completedCommand
	}{
		{
			name:     "successful command without identity",
			duration: 1500 * time.Millisecond,
			want: completedCommand{
				Command:         testCmdWorkflowList,
				DurationMs:      1500,
				Success:         true,
				ControlPlaneURL: testCPURL,
			},
		},
		{
			name:     "successful command with identity and org",
			duration: 250 * time.Millisecond,
			identity: userIdentity,
			orgName:  testOrgName,
			want: completedCommand{
				Command:          testCmdWorkflowList,
				DurationMs:       250,
				Success:          true,
				ControlPlaneURL:  testCPURL,
				OrganizationName: testOrgName,
				TokenType:        v1.Attestation_Auth_AUTH_TYPE_USER.String(),
				UserID:           userIdentity.ID,
				OrgID:            userIdentity.OrgID,
			},
		},
		{
			name:     "failed command records the error kind",
			duration: 3 * time.Second,
			runErr:   status.Error(codes.Unavailable, "control plane is down"),
			want: completedCommand{
				Command:         testCmdWorkflowList,
				DurationMs:      3000,
				Success:         false,
				ErrorKind:       "Unavailable",
				ControlPlaneURL: testCPURL,
			},
		},
		{
			name:     "sub-millisecond command does not report a negative duration",
			duration: 10 * time.Microsecond,
			want: completedCommand{
				Command:         testCmdWorkflowList,
				DurationMs:      0,
				Success:         true,
				ControlPlaneURL: testCPURL,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd := &cobra.Command{Use: appName}
			workflowCmd := &cobra.Command{Use: testCmdWorkflow}
			listCmd := &cobra.Command{Use: testCmdList}
			workflowCmd.AddCommand(listCmd)
			rootCmd.AddCommand(workflowCmd)

			got := buildCompletedCommand(listCmd, tc.duration, tc.runErr, tc.identity, testCPURL, tc.orgName)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestBuildCompletedCommandOmitsTheToken is the guardrail for the payload crossing a
// process boundary: it must carry the parsed identity, never anything token-shaped.
func TestBuildCompletedCommandOmitsTheToken(t *testing.T) {
	rootCmd := &cobra.Command{Use: appName}
	child := &cobra.Command{Use: testCmdList}
	rootCmd.AddCommand(child)

	payload := buildCompletedCommand(child, time.Second, nil, &token.ParsedToken{
		ID:        "user-id",
		OrgID:     "org-id",
		TokenType: v1.Attestation_Auth_AUTH_TYPE_API_TOKEN,
	}, testCPURL, "")

	encoded, err := json.Marshal(payload)
	assert.NoError(t, err)
	assert.NotContains(t, string(encoded), "eyJ") // a JWT always starts with this
	assert.Contains(t, string(encoded), "user-id")
}

func TestExtractCmdLineFromCommand(t *testing.T) {
	rootCmd := &cobra.Command{Use: appName}
	workflowCmd := &cobra.Command{Use: testCmdWorkflow}
	listCmd := &cobra.Command{Use: testCmdList}
	workflowCmd.AddCommand(listCmd)
	rootCmd.AddCommand(workflowCmd)

	testCases := []struct {
		name string
		cmd  *cobra.Command
		want string
	}{
		{
			name: "nested command",
			cmd:  listCmd,
			want: testCmdWorkflowList,
		},
		{
			name: "top level command",
			cmd:  workflowCmd,
			want: testCmdWorkflow,
		},
		{
			name: "the root command itself has no hierarchy",
			cmd:  rootCmd,
			want: "",
		},
		{
			// This now runs on the exit path of failed commands too, so a command that never
			// got attached to the root must end the walk rather than panic on a nil parent.
			name: "command detached from the root",
			cmd:  &cobra.Command{Use: "orphan"},
			want: "orphan",
		},
		{
			name: "no command",
			cmd:  nil,
			want: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				assert.Equal(t, tc.want, extractCmdLineFromCommand(tc.cmd))
			})
		})
	}
}

func TestShouldReportCommand(t *testing.T) {
	testCases := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{
			name: "regular command is reported",
			want: true,
		},
		{
			name:        "local-only command is not reported",
			annotations: map[string]string{skipActionOptsInit: trueString},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: testCmdList, Annotations: tc.annotations}
			assert.Equal(t, tc.want, shouldReportCommand(cmd))
		})
	}
}

// TestShouldReportCommandSkipsTheFlushCommand keeps the child from reporting itself,
// which would make every command produce an endless chain of processes. The guard must
// hold even if the annotation the rest of the rule reads is taken off the command.
func TestShouldReportCommandSkipsTheFlushCommand(t *testing.T) {
	rootCmd := &cobra.Command{Use: appName}
	rootCmd.AddCommand(newTelemetryCmd())

	flushCmd, _, err := rootCmd.Find([]string{telemetryCmdUse, telemetryFlushCmdUse})
	require.NoError(t, err)
	require.Equal(t, telemetryFlushCmdUse, flushCmd.Use)
	assert.False(t, shouldReportCommand(flushCmd))

	flushCmd.Annotations = nil
	assert.False(t, shouldReportCommand(flushCmd), "the recursion guard must not depend on the annotation")
}

// TestShouldReportCommandSkipsCompletion covers the command the shell runs on every press
// of the Tab key. Reporting it would count keystrokes as commands, and each one would fork
// a delivery process.
func TestShouldReportCommandSkipsCompletion(t *testing.T) {
	// Built by hand rather than through cobra: the hidden __complete command is installed
	// during ExecuteC, so it does not exist on a root command that has not been run.
	testCases := []struct {
		name string
		use  string
	}{
		{name: "the hidden command the shell calls on Tab", use: cobra.ShellCompRequestCmd},
		{name: "the description-less variant", use: cobra.ShellCompNoDescRequestCmd},
		{name: "the user-facing completion command", use: "completion"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd := &cobra.Command{Use: appName}
			completionCmd := &cobra.Command{Use: tc.use}
			rootCmd.AddCommand(completionCmd)
			assert.False(t, shouldReportCommand(completionCmd))

			// The per-shell subcommands, such as `completion zsh`, sit under it.
			shellCmd := &cobra.Command{Use: "zsh"}
			completionCmd.AddCommand(shellCmd)
			assert.False(t, shouldReportCommand(shellCmd))
		})
	}
}
