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

package dispatcher

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	mockedSDK "github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setMaxDispatchElapsedTimeForTest temporarily shrinks the total retry
// budget so tests don't wait for the real 5m. The returned func restores
// the production value.
func setMaxDispatchElapsedTimeForTest(d time.Duration) func() {
	prev := maxDispatchElapsedTime
	maxDispatchElapsedTime = d
	return func() { maxDispatchElapsedTime = prev }
}

// TestDispatchPerAttemptTimeout verifies that each Execute call runs under
// a per-attempt deadline rather than the unbounded context.TODO() that was
// previously passed through. The mock always fails so the retry loop fires
// multiple attempts; every observed context must carry a deadline within
// the perAttemptTimeout ballpark.
func TestDispatchPerAttemptTimeout(t *testing.T) {
	// MaxElapsedTime must exceed the first backoff interval (~500ms ± jitter)
	// so the retry loop fires at least 2 attempts. 2s yields 2-3 attempts
	// with enough headroom to avoid flakes on loaded CI runners.
	restore := setMaxDispatchElapsedTimeForTest(2 * time.Second)
	t.Cleanup(restore)

	plugin := mockedSDK.NewFanOut(t)
	plugin.On("String", mock.Anything).Return("test-plugin").Maybe()

	type attemptInfo struct {
		hasDeadline bool
		deadline    time.Duration
	}
	var observed []attemptInfo

	plugin.On("Execute", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		ctx := args.Get(0).(context.Context)
		deadline, ok := ctx.Deadline()
		observed = append(observed, attemptInfo{
			hasDeadline: ok,
			deadline:    time.Until(deadline),
		})
	}).Return(errors.New("transient failure"))

	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	opts := &sdk.ExecutionRequest{
		Input: &sdk.ExecuteInput{
			Attestation: &sdk.ExecuteAttestation{},
		},
	}

	err := dispatch(context.Background(), plugin, opts, logger)
	require.Error(t, err)

	require.GreaterOrEqual(t, len(observed), 2, "should have retried at least once")

	// Every attempt must have a per-attempt deadline near perAttemptTimeout.
	// No attempt should inherit an unbounded (no-deadline) context.
	for i, a := range observed {
		assert.True(t, a.hasDeadline, "attempt %d had no deadline", i)
		assert.Greater(t, a.deadline, 0*time.Second, "attempt %d deadline already exceeded", i)
		assert.Less(t, a.deadline, sdk.PerAttemptTimeout+1*time.Second,
			"attempt %d deadline %s exceeded perAttemptTimeout+slack", i, a.deadline)
	}
}

// TestDispatchSucceedsOnRetry verifies that a transient failure followed by
// success stops the retry loop and returns nil.
func TestDispatchSucceedsOnRetry(t *testing.T) {
	// Safety net: if the mock setup is wrong, fail fast instead of waiting 5m.
	restore := setMaxDispatchElapsedTimeForTest(2 * time.Second)
	t.Cleanup(restore)

	plugin := mockedSDK.NewFanOut(t)
	plugin.On("String", mock.Anything).Return("test-plugin").Maybe()

	// First call fails, second call succeeds. .Once() ensures each
	// expectation is consumed exactly once, so the second call returns nil.
	plugin.On("Execute", mock.Anything, mock.Anything).Return(errors.New("transient failure")).Once()
	plugin.On("Execute", mock.Anything, mock.Anything).Return(nil).Once()

	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	opts := &sdk.ExecutionRequest{
		Input: &sdk.ExecuteInput{
			Attestation: &sdk.ExecuteAttestation{},
		},
	}

	err := dispatch(context.Background(), plugin, opts, logger)
	require.NoError(t, err)

	var execCalls int
	for _, c := range plugin.Calls {
		if c.Method == "Execute" {
			execCalls++
		}
	}
	assert.Equal(t, 2, execCalls, "should have called Execute exactly twice")
}

// TestDispatchNoInput verifies the fast-fail path that doesn't enter the retry loop.
func TestDispatchNoInput(t *testing.T) {
	plugin := mockedSDK.NewFanOut(t)
	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	opts := &sdk.ExecutionRequest{Input: &sdk.ExecuteInput{}}

	err := dispatch(context.Background(), plugin, opts, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no input provided")
	plugin.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

// TestDispatchRespectsParentContextCancellation verifies that cancelling the
// parent context stops the retry loop promptly, rather than waiting for the
// full MaxElapsedTime budget to expire.
func TestDispatchRespectsParentContextCancellation(t *testing.T) {
	restore := setMaxDispatchElapsedTimeForTest(30 * time.Second)
	t.Cleanup(restore)

	plugin := mockedSDK.NewFanOut(t)
	plugin.On("String", mock.Anything).Return("test-plugin").Maybe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel the parent context on the first Execute call so the retry loop
	// should stop during the backoff sleep that follows.
	plugin.On("Execute", mock.Anything, mock.Anything).Run(func(_ mock.Arguments) {
		cancel()
	}).Return(errors.New("transient failure"))

	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	opts := &sdk.ExecutionRequest{
		Input: &sdk.ExecuteInput{
			Attestation: &sdk.ExecuteAttestation{},
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- dispatch(ctx, plugin, opts, logger)
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		// The loop should exit with the parent context's cancellation error,
		// not keep retrying for the full 30s budget.
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		cancel() // unblock the goroutine
		err := <-done
		assert.Failf(t, "dispatch did not return within 5s of parent context cancellation",
			"eventually returned: %v", err)
	}
}
