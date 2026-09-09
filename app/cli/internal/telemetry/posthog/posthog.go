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

package posthog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/chainloop-dev/chainloop/app/cli/internal/telemetry"
	"github.com/posthog/posthog-go"
)

var _ telemetry.Client = (*Tracker)(nil)

var ErrInvalidConfig = errors.New("invalid configuration, API Key and endpoint URL are required")

// NewClient creates a new PosthogTracker instance.
func NewClient(apiKey string, endpointURL string) (*Tracker, error) {
	if apiKey == "" || endpointURL == "" {
		return nil, ErrInvalidConfig
	}

	noopLogger := log.New(io.Discard, "", 0)
	client, err := posthog.NewWithConfig(apiKey, posthog.Config{
		Endpoint: endpointURL,
		Logger:   posthog.StdLogger(noopLogger, false),
		// Close is what flushes the batch, and it waits indefinitely unless bounded.
		ShutdownTimeout: telemetry.FlushTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create PostHog client: %w", err)
	}

	return &Tracker{
		client: client,
	}, nil
}

// enqueuer is the subset of posthog.Client that the Tracker uses. Depending on the
// narrow interface instead of the full SDK one keeps the tracker testable: the SDK
// interface also carries the feature-flag surface, which this package never touches.
type enqueuer interface {
	posthog.EnqueueClient
	Close() error
}

// Tracker is an implementation of the telemetry.Client interface for PostHog.
type Tracker struct {
	client enqueuer
}

// TrackEvent sends an event to the PostHog server.
func (p *Tracker) TrackEvent(_ context.Context, eventName string, id string, tags telemetry.Tags) error {
	if p == nil {
		return nil
	}

	defer p.client.Close()
	msg := posthog.Capture{
		DistinctId: id,
		Event:      eventName,
		Properties: posthog.NewProperties(),
	}

	// Set the tags as properties.
	for k, v := range tags {
		msg.Properties.Set(k, v)
	}

	// Both groups belong on the same event: cp_installation identifies the control plane the
	// command talked to, organization the tenant within it. They are set on a single Groups
	// map because replacing the map would drop whichever group was assigned first.
	groups := posthog.NewGroups()
	if tags["cp_url_hash"] != "" {
		groups.Set("cp_installation", tags["cp_url_hash"])
	}
	if tags["org_id"] != "" {
		groups.Set("organization", tags["org_id"])
	}
	if len(groups) > 0 {
		msg.Groups = groups
	}

	// Alias the machine ID onto the account so the events a user produced before logging in
	// still resolve to them. Only interactive user sessions qualify: API and federated tokens
	// are shared identities running on ephemeral CI machines, where aliasing merges unrelated
	// runners into a single person and emits an alias on every command.
	if tags.IsInteractiveUserSession() &&
		(tags["machine_id"] != "" && tags["machine_id"] != id) && id != telemetry.UnrecognisedUserID {
		if err := p.client.Enqueue(posthog.Alias{
			DistinctId: id,
			Alias:      tags["machine_id"],
		}); err != nil {
			return fmt.Errorf("failed to track event: %w", err)
		}
	}

	// Enqueue the event.
	err := p.client.Enqueue(msg)
	if err != nil {
		return fmt.Errorf("failed to track event: %w", err)
	}

	return nil
}
