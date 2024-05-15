//
// Copyright 2024 The Chainloop Authors.
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

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	"github.com/denisbrodbeck/machineid"
	"github.com/rs/zerolog"
)

const commandTrackerEventName = "command_executed"

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
	if t.client == nil {
		return nil
	}

	computedTags := loadDefaultTags()
	// Add on top of the default tags the ones passed as argument.
	// TODO: Add a way to avoid the override default tags.
	for k, v := range tags {
		computedTags[k] = v
	}
	computedTags["command"] = cmd

	var id string
	id, err := machineid.ProtectedID("chainloop")
	if err != nil {
		// If the machine ID is not available, use a default ID otherwise the underlying library will fail.
		id = "default-id"
	}

	return t.client.TrackEvent(ctx, commandTrackerEventName, id, computedTags)
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
	runner := crafter.DiscoverRunner(zerolog.Nop())
	tg["ci"] = "false"
	// Check if the ID of the runner matches the unspecified one, meaning it's the Generic Runner
	if runner.ID() != schemaapi.CraftingSchema_Runner_RUNNER_TYPE_UNSPECIFIED {
		tg["ci"] = "true"
		// TODO: Add more environment information for each individual CI system
		tg["runner"] = runner.ID().String()
	}

	return tg
}
