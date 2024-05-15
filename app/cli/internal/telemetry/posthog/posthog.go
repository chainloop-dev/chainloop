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
	msg := posthog.Capture{
		DistinctId: id,
		Event:      eventName,
	}

	for k, v := range tags {
		msg.Properties.Set(k, v)
	}

	return p.client.Enqueue(msg)
}
