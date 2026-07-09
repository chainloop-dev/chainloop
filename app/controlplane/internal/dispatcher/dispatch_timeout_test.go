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
