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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/chainloop-dev/chainloop/app/cli/internal/telemetry"
	"github.com/chainloop-dev/chainloop/app/cli/internal/telemetry/posthog"
	"github.com/spf13/cobra"
)

const (
	telemetryCmdUse      = "telemetry"
	telemetryFlushCmdUse = "flush"

	// maxTelemetryPayloadBytes bounds what the flush command will read from stdin. A real
	// payload is a few hundred bytes; the limit is generous enough that a long control
	// plane URL or organization name cannot reach it.
	maxTelemetryPayloadBytes = 64 * 1024
)

// newTelemetryCmd builds the internal command tree the parent process re-execs into. It is
// hidden because it is not something a user ever runs: `chainloop telemetry flush` is how a
// finished command gets its event delivered without the user waiting for the network.
func newTelemetryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    telemetryCmdUse,
		Short:  "Internal: deliver CLI telemetry",
		Hidden: true,
		Annotations: map[string]string{
			skipActionOptsInit: trueString,
		},
	}

	cmd.AddCommand(newTelemetryFlushCmd())

	return cmd
}

func newTelemetryFlushCmd() *cobra.Command {
	return &cobra.Command{
		Use:    telemetryFlushCmdUse,
		Short:  "Internal: deliver a single telemetry event read from stdin",
		Hidden: true,
		Annotations: map[string]string{
			skipActionOptsInit: trueString,
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := posthog.NewClient(posthogAPIKey, posthogEndpoint)
			if err != nil {
				logger.Debug().Err(err).Msg("creating telemetry client")
				return nil
			}

			if err := runTelemetryFlush(os.Stdin, client); err != nil {
				logger.Debug().Err(err).Msg("delivering telemetry event")
			}

			// Always succeed. Nobody is reading this process' exit code, and a non-zero one
			// would only show up as noise if the child were ever run in the foreground.
			return nil
		},
	}
}

// runTelemetryFlush reads one completedCommand payload and delivers it as the
// command_executed event. It rebuilds the same tag set the CLI has always reported and adds
// what only the finished command knew: how long it took and whether it worked.
//
// Everything derivable from the machine (OS, arch, CI runner, machine ID) is computed here
// rather than carried in the payload, because this runs on the same machine, in the same
// environment, from the same binary as the command it is reporting.
func runTelemetryFlush(r io.Reader, client telemetry.Client) error {
	// The payload is a few hundred bytes and comes from this CLI's own parent process, but
	// stdin is stdin: bound the read rather than trust it.
	decoder := json.NewDecoder(io.LimitReader(r, maxTelemetryPayloadBytes))
	// A field this binary does not know about means the payload came from a different
	// version of the CLI, which only happens if the binary was replaced mid-command. Fail
	// loudly into the debug log rather than silently reporting a half-understood event.
	decoder.DisallowUnknownFields()

	var payload completedCommand
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("decoding telemetry payload: %w", err)
	}

	tags := telemetry.Tags{
		"cli_version":         Version,
		"edition":             Edition,
		"cp_url_hash":         hashURL(payload.ControlPlaneURL),
		"cp_installation_url": payload.ControlPlaneURL,
		"chainloop_source":    "cli",
		"duration_ms":         payload.DurationMs,
		"success":             payload.Success,
	}

	for key, value := range map[string]string{
		"error_kind":        payload.ErrorKind,
		"organization_name": payload.OrganizationName,
		"token_type":        payload.TokenType,
		"user_id":           payload.UserID,
		"org_id":            payload.OrgID,
	} {
		if value != "" {
			tags[key] = value
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), telemetry.FlushTimeout)
	defer cancel()

	if err := telemetry.NewCommandTracker(client).Track(ctx, payload.Command, tags); err != nil {
		return fmt.Errorf("sending event: %w", err)
	}

	return nil
}
