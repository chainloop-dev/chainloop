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
	OnlyDefaults bool
	// Interval between checks, defaults to 30 minutes
	CheckInterval time.Duration
	// Timeout for each individual backend validation, defaults to 10 seconds
	ValidationTimeout time.Duration
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

	onlyDefaults := true
	if opts != nil {
		onlyDefaults = opts.OnlyDefaults
	}

	// Apply validation timeout from options if provided
	if opts != nil && opts.ValidationTimeout > 0 {
		c.validationTimeout = opts.ValidationTimeout
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run one check immediately
	if err := c.CheckAllBackends(ctx, onlyDefaults); err != nil {
		c.logger.Errorf("initial CAS backend check failed: %v", err)
	}

	c.logger.Infof("CAS backend checker started with interval %s, checking %s, timeout %s",
		interval,
		conditionalString(onlyDefaults, "only default backends", "all backends"),
		c.validationTimeout)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("CAS backend checker stopping due to context cancellation")
			return
		case <-ticker.C:
			if err := c.CheckAllBackends(ctx, onlyDefaults); err != nil {
				c.logger.Errorf("periodic CAS backend check failed: %v", err)
			}
		}
	}
}

// CheckAllBackends validates all CAS backends (or just default ones based on configuration)
// using a worker pool for parallel processing with timeouts
func (c *CASBackendChecker) CheckAllBackends(ctx context.Context, onlyDefaults bool) error {
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

// Helper function to return different strings based on a condition
func conditionalString(condition bool, trueStr, falseStr string) string {
	if condition {
		return trueStr
	}
	return falseStr
}
