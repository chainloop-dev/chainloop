//
// Copyright 2024-2026 The Chainloop Authors.
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

package telemetry

import (
	"context"
	"runtime"
	"time"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	"github.com/denisbrodbeck/machineid"
	"github.com/rs/zerolog"
)

const commandTrackerEventName = "command_executed"
const UnrecognisedUserID = "unrecognised"

// FlushTimeout bounds how long the CLI spends delivering a telemetry event. The PostHog
// client takes it as the deadline for flushing its batch on close, and the command hook
// takes it as the deadline after which it stops waiting on the delivery goroutine. Both
// read it from here so the two deadlines cannot drift apart.
const FlushTimeout = 2 * time.Second

// authTypeUser mirrors v1.Attestation_Auth_AUTH_TYPE_USER.String(). It is duplicated as a
// literal so this package keeps no dependency on the attestation API; a test pins the two
// values together.
const authTypeUser = "AUTH_TYPE_USER"

// Tags represents a collection of event tags.
type Tags map[string]string

// Client defines the interface for tracking events.
type Client interface {
	TrackEvent(ctx context.Context, eventName string, id string, tags Tags) error
}

// CommandTracker is an implementation in charge of tracking Commands events sent from the CLI.
type CommandTracker struct {
	client Client
}

// NewCommandTracker creates a new CommandTracker instance.
func NewCommandTracker(client Client) *CommandTracker {
	return &CommandTracker{
		client: client,
	}
}

// Track sends a command event to the telemetry.Client.
func (t *CommandTracker) Track(ctx context.Context, cmd string, tags Tags) error {
	// Ensure a valid client is available.
	if t.client == nil {
		return nil
	}

	// Load default tags and merge with user-provided tags.
	computedTags := mergeTags(loadDefaultTags(), tags)

	// Set the command tag.
	computedTags["command"] = cmd

	// Determine the user ID.
	id := determineUserID(computedTags)

	// Track the event with computed tags.
	return t.client.TrackEvent(ctx, commandTrackerEventName, id, computedTags)
}

// IsInteractiveUserSession reports whether the command was run by a logged-in human outside
// of CI. API and federated tokens are shared identities: an API token belongs to an
// organization rather than a person, and a federated token resolves to the issuer URL, which
// is the same value for every Chainloop installation using that provider. Anything derived
// from the machine the command ran on is only meaningful for the interactive case.
func (tg Tags) IsInteractiveUserSession() bool {
	return tg["token_type"] == authTypeUser && tg["ci"] == "false"
}

// Merges two tag maps.
func mergeTags(defaultTags, userTags Tags) Tags {
	result := make(Tags)
	for k, v := range defaultTags {
		result[k] = v
	}
	for k, v := range userTags {
		result[k] = v
	}
	return result
}

// determineUserID Determines the user ID using available information or generates a random one.
func determineUserID(tags Tags) string {
	// Get the machine ID.
	machineID, _ := machineid.ProtectedID("chainloop")
	tags["machine_id"] = machineID

	// Check if user ID is provided in tags.
	// This won't happen in the unauthenticated case scenario.
	if userID, ok := tags["user_id"]; ok && userID != "" {
		return userID
	}

	// If machine ID is available, return it.
	if machineID != "" {
		return machineID
	}

	// Return an unrecognised user ID.
	return UnrecognisedUserID
}

// loadDefaultTags returns a map of default tags that are added to every event.
func loadDefaultTags() Tags {
	return Tags{}.WithRuntimeInfo().WithEnvironmentInfo()
}

// WithRuntimeInfo adds runtime information to the Tags.
func (tg Tags) WithRuntimeInfo() Tags {
	tg["os"] = runtime.GOOS
	tg["arch"] = runtime.GOARCH

	return tg
}

// WithEnvironmentInfo adds environment information to the Tags.
func (tg Tags) WithEnvironmentInfo() Tags {
	runner := crafter.DiscoverRunner("", zerolog.Nop())
	tg["ci"] = "false"
	// Check if the ID of the runner matches the unspecified one, meaning it's the Generic Runner
	if runner.ID() != schemaapi.CraftingSchema_Runner_RUNNER_TYPE_UNSPECIFIED {
		tg["ci"] = "true"
		// TODO: Add more environment information for each individual CI system
		tg["runner"] = runner.ID().String()
	}

	return tg
}
