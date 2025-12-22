//
// Copyright 2025 The Chainloop Authors.
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
)

// Default check interval if none is provided
const (
	defaultInterval          = 30 * time.Minute
	defaultValidationTimeout = 10 * time.Second
)

type CASBackendChecker struct {
	logger             *log.Helper
	casBackendRepo     CASBackendRepo
	caseBackendUseCase *CASBackendUseCase
	// Validation timeout for each backend check
	validationTimeout time.Duration
}

type CASBackendCheckerOpts struct {
	// Whether to check only default backends or all backends
	OnlyDefaultsOrFallbacks *bool
	// Interval between checks, defaults to 30 minutes
	CheckInterval time.Duration
	// Timeout for each individual backend validation, defaults to 10 seconds
	ValidationTimeout time.Duration
	// Initial delay before first validation (includes jitter). If not set, runs immediately.
	InitialDelay time.Duration
}

// NewCASBackendChecker creates a new CAS backend checker that will periodically validate
// the status of CAS backends
func NewCASBackendChecker(logger log.Logger, casBackendRepo CASBackendRepo, casBackendUseCase *CASBackendUseCase) *CASBackendChecker {
	return &CASBackendChecker{
		logger:             log.NewHelper(log.With(logger, "component", "biz/CASBackendChecker")),
		casBackendRepo:     casBackendRepo,
		caseBackendUseCase: casBackendUseCase,
		validationTimeout:  defaultValidationTimeout,
	}
}

// Start begins the periodic checking of CAS backends
func (c *CASBackendChecker) Start(ctx context.Context, opts *CASBackendCheckerOpts) {
	interval := defaultInterval
	if opts != nil && opts.CheckInterval > 0 {
		interval = opts.CheckInterval
	}

	onlyDefaultsOrFallbacks := true
	if opts != nil && opts.OnlyDefaultsOrFallbacks != nil {
		onlyDefaultsOrFallbacks = *opts.OnlyDefaultsOrFallbacks
	}

	// Apply validation timeout from options if provided
	if opts != nil && opts.ValidationTimeout > 0 {
		c.validationTimeout = opts.ValidationTimeout
	}

	// Apply initial delay from options if provided
	var initialDelay = 0 * time.Second
	if opts != nil && opts.InitialDelay > 0 {
		initialDelay = opts.InitialDelay
	}

	c.logger.Infow("msg", "CAS backend checker configured", "initialDelay", initialDelay, "interval", interval, "allBackends", !onlyDefaultsOrFallbacks, "timeout", c.validationTimeout)

	select {
	case <-ctx.Done():
		c.logger.Info("CAS backend checker stopping due to context cancellation before initial check")
		return
	case <-time.After(initialDelay):
		// Continue to first check
	}

	// Run first check
	if err := c.checkBackends(ctx, onlyDefaultsOrFallbacks); err != nil {
		c.logger.Errorf("initial CAS backend check failed: %v", err)
	}

	// Start periodic checks
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("CAS backend checker stopping due to context cancellation")
			return
		case <-ticker.C:
			if err := c.checkBackends(ctx, onlyDefaultsOrFallbacks); err != nil {
				c.logger.Errorf("periodic CAS backend check failed: %v", err)
			}
		}
	}
}

// checkBackends validates all CAS backends (or just default ones based on configuration)
// using a worker pool for parallel processing with timeouts
func (c *CASBackendChecker) checkBackends(ctx context.Context, onlyDefaults bool) error {
	c.logger.Debug("starting CAS backend validation check")

	backends, err := c.casBackendRepo.ListBackends(ctx, onlyDefaults)
	if err != nil {
		return fmt.Errorf("failed to list CAS backends: %w", err)
	}

	c.logger.Debugf("found %d CAS backends to validate using %s timeout per backend",
		len(backends), c.validationTimeout)

	if len(backends) == 0 {
		return nil
	}

	for _, backend := range backends {
		// Create a context with timeout for this specific backend validation
		timeoutCtx, cancel := context.WithTimeout(ctx, c.validationTimeout)

		c.logger.Debugf("validating CAS backend %s (%s)", backend.ID, backend.Name)

		// Run the validation and log the result
		err := c.caseBackendUseCase.PerformValidation(timeoutCtx, backend.ID.String())
		if err != nil {
			c.logger.Errorf("failed to validate CAS backend %s: %v", backend.ID, err)
		} else {
			c.logger.Debugf("successfully validated CAS backend %s", backend.ID)
		}

		// Clean up the timeout context
		cancel()
	}

	c.logger.Debug("all CAS backend validations completed")
	return nil
}
