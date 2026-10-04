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
	"os"
	"os/exec"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/token"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc/status"
)

// errorKindOther is the error_kind reported for anything that is not a gRPC status error.
// The error message itself is never reported: it routinely carries file paths, organization
// names and material names.
const errorKindOther = "other"

// completedCommand is the payload the parent hands to `chainloop telemetry flush`. It
// carries only what the child cannot work out for itself: the child runs the same binary
// on the same machine, so it derives version, edition, OS, arch, CI runner and machine ID,
// but it never sees the flags and config that produced the control plane URL and the
// organization name.
//
// The auth token is deliberately absent. The identity fields are the output of token.Parse,
// which is all telemetry has ever reported, so the credential never crosses the boundary.
type completedCommand struct {
	Command          string `json:"command"`
	DurationMs       int64  `json:"duration_ms"`
	Success          bool   `json:"success"`
	ErrorKind        string `json:"error_kind,omitempty"`
	ControlPlaneURL  string `json:"cp_url"`
	OrganizationName string `json:"organization_name,omitempty"`
	TokenType        string `json:"token_type,omitempty"`
	UserID           string `json:"user_id,omitempty"`
	OrgID            string `json:"org_id,omitempty"`
}

// reportCommand delivers the telemetry event for a command that has just finished.
//
// The duration is only knowable here, after the command returned, and the process is about
// to exit, so there is no time left to wait on the network. Delivery is therefore handed to
// a detached child process and this function returns as soon as that child is spawned. Any
// failure is swallowed at debug level: telemetry never affects the outcome of a command.
func reportCommand(executed *cobra.Command, d time.Duration, runErr error) {
	if isTelemetryDisabled() || !shouldReportCommand(executed) {
		return
	}

	logger.Debug().Msg("Telemetry enabled, to disable it use DO_NOT_TRACK=1")

	payload := buildCompletedCommand(executed, d, runErr, telemetryIdentity(),
		controlPlaneURL(), viper.GetString(confOptions.organization.viperKey))

	// An empty command path means cobra resolved nothing to run: `chainloop` on its own,
	// which prints help, or an unrecognised subcommand. Those are not commands anyone ran,
	// and reporting them would file every one of them under the same empty name.
	if payload.Command == "" {
		return
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		logger.Debug().Err(err).Msg("encoding telemetry payload")
		return
	}

	if err := spawnTelemetryFlush(encoded); err != nil {
		logger.Debug().Err(err).Msg("sending command to telemetry")
	}
}

// telemetryIdentity reduces the credentials in use to the identity telemetry reports: token
// type, user and organization. It reads the token from ActionOpts, which the pre-run hook
// already populated, so nothing has to be parked in a variable for the exit path. It
// returns nil for unauthenticated commands, for the local-only commands that never build
// ActionOpts, and when the token does not parse, all of which report without identity tags.
func telemetryIdentity() *token.ParsedToken {
	if ActionOpts == nil {
		return nil
	}

	identity, err := token.Parse(ActionOpts.AuthTokenRaw)
	if err != nil {
		logger.Debug().Err(err).Msg("parsing token for telemetry")
		return nil
	}

	return identity
}

// shouldReportCommand reports whether a command produces a telemetry event.
//
// The telemetry subtree is excluded first and on its own terms: `telemetry flush` is the
// child that delivers an event, so reporting it would make every command spawn a child
// that spawns a child, without end. That guarantee must not rest on an annotation that
// means something else.
//
// Shell completion is excluded because the shell runs it on every press of the Tab key.
// Those are keystrokes rather than commands, and counting them inflates the command
// totals with something the user never ran.
//
// Beyond that, commands annotated skipActionOptsInit are the local-only ones, such as the
// git hooks, which have never been reported.
func shouldReportCommand(executed *cobra.Command) bool {
	if executed == nil || isTelemetryCommand(executed) || isCompletionCommand(executed) {
		return false
	}

	return executed.Annotations[skipActionOptsInit] != trueString
}

// isCompletionCommand reports whether a command is one of the shell-completion entry
// points cobra installs, either the hidden one the shell calls on every Tab or the one a
// user runs to install a completion script.
func isCompletionCommand(executed *cobra.Command) bool {
	for cmd := executed; cmd != nil; cmd = cmd.Parent() {
		switch cmd.Name() {
		case cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd, "completion":
			return true
		}
	}

	return false
}

// isTelemetryCommand reports whether a command belongs to the internal telemetry subtree.
func isTelemetryCommand(executed *cobra.Command) bool {
	for cmd := executed; cmd != nil; cmd = cmd.Parent() {
		if cmd.Use == telemetryCmdUse {
			return true
		}
	}

	return false
}

func buildCompletedCommand(executed *cobra.Command, d time.Duration, runErr error,
	identity *token.ParsedToken, controlplaneURL, orgName string,
) completedCommand {
	payload := completedCommand{
		Command:          extractCmdLineFromCommand(executed),
		DurationMs:       d.Milliseconds(),
		Success:          runErr == nil,
		ErrorKind:        classifyError(runErr),
		ControlPlaneURL:  controlplaneURL,
		OrganizationName: orgName,
	}

	if identity != nil {
		payload.TokenType = identity.TokenType.String()
		payload.UserID = identity.ID
		payload.OrgID = identity.OrgID
	}

	return payload
}

// classifyError reduces an error to a coarse class: the gRPC status code name when the
// chain carries one, and "other" otherwise. It is deliberately lossy. Knowing that a
// command failed with Unavailable rather than PermissionDenied separates a control plane
// outage from an expired token without reporting anything the user typed or named.
func classifyError(err error) string {
	if err == nil {
		return ""
	}

	// errors.AsType rather than status.FromError: the CLI wraps gRPC errors as it passes
	// them up, and FromError only recognises an unwrapped one.
	type grpcstatus interface {
		error
		GRPCStatus() *status.Status
	}
	if gs, ok := errors.AsType[grpcstatus](err); ok {
		return gs.GRPCStatus().Code().String()
	}

	return errorKindOther
}

// telemetryFlushCommand resolves the command that delivers an event: this same binary,
// re-executed as `chainloop telemetry flush`. It is a variable so a test can point the
// spawn at a stub and assert on what the child actually receives.
var telemetryFlushCommand = func() (string, []string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", nil, err
	}

	return executable, []string{telemetryCmdUse, telemetryFlushCmdUse}, nil
}

// spawnTelemetryFlush starts a detached `chainloop telemetry flush` carrying the payload on
// its stdin, and returns without waiting for it.
//
// The payload travels through an anonymous pipe rather than argv so that the user and
// organization IDs never show up in the process list. Passing the read end as an *os.File
// hands the descriptor straight to the child, with no copying goroutine in this process to
// outlive, so the parent can exit the moment this returns. Payloads are a few hundred
// bytes, far below the pipe buffer, so the write never blocks.
func spawnTelemetryFlush(payload []byte) error {
	name, args, err := telemetryFlushCommand()
	if err != nil {
		return fmt.Errorf("locating executable: %w", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("creating telemetry pipe: %w", err)
	}
	// The child reads until EOF, which only arrives once the write end is closed.
	defer func() { _ = w.Close() }()

	child := exec.Command(name, args...) // nosemgrep
	child.Stdin = r
	// Detach from this process' terminal and session so a shell hang-up, job control, or a
	// CI runner reaping the step's process group cannot take the child down with it.
	child.SysProcAttr = detachSysProcAttr()

	startErr := child.Start()
	// Drop this process' copy of the read end as soon as the child has its own, whether or
	// not the spawn worked. Holding it open would make this process a reader of its own
	// pipe, so a child that died early would not break the write below: it would fill the
	// pipe buffer instead, and a payload larger than that buffer would block here, on the
	// exit path of every command.
	_ = r.Close()

	if startErr != nil {
		return fmt.Errorf("starting telemetry flush: %w", startErr)
	}

	// Release rather than Wait: the child outlives this process on purpose. Deferred so it
	// still happens if the write below fails.
	defer func() { _ = child.Process.Release() }()

	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("writing telemetry payload: %w", err)
	}

	return nil
}
