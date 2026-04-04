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

package natsconn

import (
	"context"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantNil bool
		wantErr bool
	}{
		{
			name:    "nil config returns nil",
			cfg:     nil,
			wantNil: true,
		},
		{
			name:    "empty URI returns nil",
			cfg:     &Config{URI: ""},
			wantNil: true,
		},
		{
			name:    "invalid URI returns error",
			cfg:     &Config{URI: "nats://invalid:99999"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rc, err := New(tc.cfg, log.DefaultLogger)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.wantNil {
				assert.Nil(t, rc)
			}
		})
	}
}

func TestSubscribeAndBroadcast(t *testing.T) {
	// Create a ReloadableConnection without an actual NATS conn —
	// Subscribe/Broadcast only manage channels, they don't use the conn.
	rc := &ReloadableConnection{
		logger: log.NewHelper(log.DefaultLogger),
	}

	ch := rc.Subscribe(t.Context())
	require.NotNil(t, ch)

	// Broadcast should send a signal
	rc.Broadcast()

	select {
	case <-ch:
		// received signal — pass
	case <-time.After(time.Second):
		require.Fail(t, "expected reconnect signal, got timeout")
	}
}

func TestBroadcastMultipleSubscribers(t *testing.T) {
	rc := &ReloadableConnection{
		logger: log.NewHelper(log.DefaultLogger),
	}

	ch1 := rc.Subscribe(t.Context())
	ch2 := rc.Subscribe(t.Context())
	ch3 := rc.Subscribe(t.Context())

	rc.Broadcast()

	for i, ch := range []<-chan struct{}{ch1, ch2, ch3} {
		select {
		case <-ch:
			// received — pass
		case <-time.After(time.Second):
			require.Failf(t, "subscriber did not receive signal", "subscriber %d", i)
		}
	}
}

func TestBroadcastNonBlocking(t *testing.T) {
	rc := &ReloadableConnection{
		logger: log.NewHelper(log.DefaultLogger),
	}

	ch := rc.Subscribe(t.Context())

	// Fill the buffered channel
	rc.Broadcast()
	// Second broadcast should not block even though channel is full
	rc.Broadcast()

	// Only one signal should be in the channel
	select {
	case <-ch:
	case <-time.After(time.Second):
		require.Fail(t, "expected signal")
	}

	// Channel should be empty now
	select {
	case <-ch:
		require.Fail(t, "expected no second signal in channel")
	default:
		// pass
	}
}

func TestSubscribeContextCancellation(t *testing.T) {
	rc := &ReloadableConnection{
		logger: log.NewHelper(log.DefaultLogger),
	}

	ctx, cancel := context.WithCancel(t.Context())
	ch := rc.Subscribe(ctx)

	// Cancel context — should unsubscribe and close channel
	cancel()

	// Wait for the goroutine to process the cancellation
	time.Sleep(50 * time.Millisecond)

	// Channel should be closed
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed after context cancellation")
	case <-time.After(time.Second):
		require.Fail(t, "channel was not closed after context cancellation")
	}

	// Verify subscriber was removed
	rc.mu.RLock()
	assert.Empty(t, rc.subscribers)
	rc.mu.RUnlock()
}

func TestNilReceiverSafety(t *testing.T) {
	var rc *ReloadableConnection

	// These should not panic
	assert.NotPanics(t, func() { rc.Broadcast() })
	assert.NotPanics(t, func() {
		ch := rc.Subscribe(context.Background())
		// nil receiver returns a closed channel
		_, ok := <-ch
		assert.False(t, ok)
	})
}
