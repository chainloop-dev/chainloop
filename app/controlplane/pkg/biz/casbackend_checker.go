//
// Copyright 2025-2026 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/go-kratos/kratos/v2/log"
)

var casBackendCheckerTracer = otelx.Tracer("chainloop-controlplane", "biz/casbackend_checker")

// Default check interval if none is provided
const (
	defaultInterval          = 30 * time.Minute
	defaultValidationTimeout = 10 * time.Second
	// Upper bound on how long a single tick is allowed to hold the
	// distributed lock. Defends against a hung validation pinning the lock
	// past one tick; the next tick will retry.
	defaultMaxTickDuration = 25 * time.Minute

	// Separate keys per scope so the two checker goroutines (defaults vs all backends)
	// don't block each other.
	lockKeyDefaultsScope = "cas-backend-checker:defaults"
	lockKeyAllScope      = "cas-backend-checker:all"
)

// DistributedLock is a best-effort, cluster-wide mutex used to make sure
// background jobs that should run on a single replica at a time aren't
// duplicated across pods.
type DistributedLock interface {
	// TryAcquire attempts to acquire the lock identified by key without
	// blocking. If acquired is true, the caller MUST invoke release when
	// done. The lock is also released automatically if the underlying
	// session is lost (e.g. pod crash).
	TryAcquire(ctx context.Context, key string) (acquired bool, release func(), err error)
}

type CASBackendChecker struct {
	logger             *log.Helper
	casBackendRepo     CASBackendRepo
	caseBackendUseCase *CASBackendUseCase
	// Validation timeout for each backend check
	validationTimeout time.Duration
	lock              DistributedLock
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
func NewCASBackendChecker(logger log.Logger, casBackendRepo CASBackendRepo, casBackendUseCase *CASBackendUseCase, lock DistributedLock) *CASBackendChecker {
	return &CASBackendChecker{
		logger:             log.NewHelper(log.With(logger, "component", "biz/CASBackendChecker")),
		casBackendRepo:     casBackendRepo,
		caseBackendUseCase: casBackendUseCase,
		validationTimeout:  defaultValidationTimeout,
		lock:               lock,
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

// checkBackends validates all CAS backends (or just default and fallback ones based on configuration)
// using a worker pool for parallel processing with timeouts
func (c *CASBackendChecker) checkBackends(ctx context.Context, defaultsOrFallbacks bool) error {
	key := lockKeyAllScope
	if defaultsOrFallbacks {
		key = lockKeyDefaultsScope
	}
	acquired, release, err := c.lock.TryAcquire(ctx, key)
	if err != nil {
		return fmt.Errorf("acquiring checker lock: %w", err)
	}
	if !acquired {
		c.logger.Debugw("msg", "another replica is running the CAS backend check, skipping", "scope", key)
		return nil
	}
	defer release()

	// Cap how long we can hold the lock. If validations hang, the next tick
	// retries instead of one stuck pod pinning the lock indefinitely.
	ctx, cancel := context.WithTimeout(ctx, defaultMaxTickDuration)
	defer cancel()

	ctx, span := otelx.Start(ctx, casBackendCheckerTracer, "CASBackendChecker.checkBackends")
	defer span.End()

	c.logger.Debug("starting CAS backend validation check")

	backends, err := c.casBackendRepo.ListBackends(ctx, defaultsOrFallbacks)
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
