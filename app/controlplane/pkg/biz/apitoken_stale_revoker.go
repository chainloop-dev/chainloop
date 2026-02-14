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

package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

const defaultSweepInterval = 1 * time.Hour

// APITokenStaleRevoker periodically scans for inactive API tokens and revokes them
// based on each organization's configured inactivity threshold.
type APITokenStaleRevoker struct {
	logger    *log.Helper
	orgRepo   OrganizationRepo
	tokenRepo APITokenRepo
}

// APITokenStaleRevokerOpts configures the sweeper's behavior.
type APITokenStaleRevokerOpts struct {
	// CheckInterval is the interval between sweeps. Defaults to 1 hour.
	CheckInterval time.Duration
	// InitialDelay is an optional delay before the first sweep.
	InitialDelay time.Duration
}

// NewAPITokenStaleRevoker creates a new stale token revoker.
func NewAPITokenStaleRevoker(orgRepo OrganizationRepo, tokenRepo APITokenRepo, logger log.Logger) *APITokenStaleRevoker {
	return &APITokenStaleRevoker{
		logger:    log.NewHelper(log.With(logger, "component", "biz/APITokenStaleRevoker")),
		orgRepo:   orgRepo,
		tokenRepo: tokenRepo,
	}
}

// Start begins the periodic sweep loop.
func (r *APITokenStaleRevoker) Start(ctx context.Context, opts *APITokenStaleRevokerOpts) {
	interval := defaultSweepInterval
	if opts != nil && opts.CheckInterval > 0 {
		interval = opts.CheckInterval
	}

	var initialDelay time.Duration
	if opts != nil && opts.InitialDelay > 0 {
		initialDelay = opts.InitialDelay
	}

	r.logger.Infow("msg", "API token stale revoker configured", "initialDelay", initialDelay, "interval", interval)

	// Wait for initial delay
	select {
	case <-ctx.Done():
		r.logger.Info("API token stale revoker stopping before initial sweep")
		return
	case <-time.After(initialDelay):
	}

	// Run first sweep
	if err := r.Sweep(ctx); err != nil {
		r.logger.Errorf("initial stale token sweep failed: %v", err)
	}

	// Start periodic sweeps
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("API token stale revoker stopping")
			return
		case <-ticker.C:
			if err := r.Sweep(ctx); err != nil {
				r.logger.Errorf("stale token sweep failed: %v", err)
			}
		}
	}
}

// Sweep finds organizations with a token inactivity threshold and revokes their stale tokens.
func (r *APITokenStaleRevoker) Sweep(ctx context.Context) error {
	orgs, err := r.orgRepo.FindWithTokenInactivityThreshold(ctx)
	if err != nil {
		return fmt.Errorf("finding organizations with inactivity threshold: %w", err)
	}

	if len(orgs) == 0 {
		r.logger.Debug("no organizations with token inactivity threshold configured")
		return nil
	}

	r.logger.Debugf("checking %d organizations for stale tokens", len(orgs))

	now := time.Now()
	for _, org := range orgs {
		if org.APITokenInactivityThresholdDays == nil {
			continue
		}

		orgID, err := uuid.Parse(org.ID)
		if err != nil {
			r.logger.Errorf("invalid org ID %s: %v", org.ID, err)
			continue
		}

		cutoff := now.Add(-time.Duration(*org.APITokenInactivityThresholdDays) * 24 * time.Hour)
		revoked, err := r.tokenRepo.RevokeInactive(ctx, orgID, cutoff)
		if err != nil {
			r.logger.Errorf("revoking stale tokens for org %s (%s): %v", org.Name, org.ID, err)
			continue
		}

		if len(revoked) > 0 {
			r.logger.Infow("msg", "revoked stale API tokens",
				"org", org.Name, "orgID", org.ID,
				"count", len(revoked),
				"thresholdDays", *org.APITokenInactivityThresholdDays,
			)
		}
	}

	return nil
}
