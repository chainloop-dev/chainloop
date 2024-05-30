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

package posthog

import (
	"context"
	"errors"
	"fmt"

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

	client, err := posthog.NewWithConfig(apiKey, posthog.Config{
		Endpoint: endpointURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create PostHog client: %w", err)
	}

	return &Tracker{
		client: client,
	}, nil
}

// Tracker is an implementation of the telemetry.Client interface for PostHog.
type Tracker struct {
	client posthog.Client
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

	// Assign the installation ID if available as a group.
	// It creates a new group named cp_installation where the values are the cp_url_hash.
	if tags["cp_url_hash"] != "" {
		msg.Groups = posthog.
			NewGroups().
			Set("cp_installation", tags["cp_url_hash"])
	}
	// It creates a new group named org_id where the values are the org_id.
	if tags["org_id"] != "" {
		msg.Groups = posthog.
			NewGroups().
			Set("organization", tags["org_id"])
	}
	// Assign an alias to the userID in the following cases:
	// - The machine ID is available and different from the userID.
	// - The userID is different from the default one.
	// An alias can help to track the same user across different devices even when it was not logged in.
	if (tags["machine_id"] != "" && tags["machine_id"] != id) && id != telemetry.UnrecognisedUserID {
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
