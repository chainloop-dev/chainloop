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

package panicguard_test

import (
	"sync"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/panicguard"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogger counts Log calls so a test can assert a recovery was logged.
type captureLogger struct {
	mu sync.Mutex
	n  int
}

func (c *captureLogger) Log(_ log.Level, _ ...interface{}) error {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return nil
}

func (c *captureLogger) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// TestRecover_ContainsPanic is the core assertion: a panicking fn returns
// normally instead of crashing the process. Without the recovery frame this
// panic would abort the test binary.
func TestRecover_ContainsPanic(t *testing.T) {
	cl := &captureLogger{}
	logger := log.NewHelper(cl)

	require.NotPanics(t, func() {
		panicguard.Recover(logger, "unit", func() {
			panic("boom")
		})
	})

	assert.Equal(t, 1, cl.count(), "the recovered panic must be logged once")
}

// TestRecover_RunsFnAndReturns confirms the happy path is untouched.
func TestRecover_RunsFnAndReturns(t *testing.T) {
	cl := &captureLogger{}
	logger := log.NewHelper(cl)

	ran := false
	panicguard.Recover(logger, "unit", func() { ran = true })

	assert.True(t, ran, "fn must run")
	assert.Equal(t, 0, cl.count(), "no panic means nothing is logged")
}

// TestGo_ContainsConcurrentPanics confirms the async wrapper contains panics in
// detached goroutines. The process surviving to the assertion is the proof.
func TestGo_ContainsConcurrentPanics(t *testing.T) {
	cl := &captureLogger{}
	logger := log.NewHelper(cl)

	const n = 50
	for i := 0; i < n; i++ {
		panicguard.Go(logger, "unit", func() { panic("boom") })
	}

	require.Eventually(t, func() bool { return cl.count() == n }, time.Second, time.Millisecond,
		"every detached panic must be recovered and logged")
}

// TestGo_NilLoggerDoesNotPanic guards the defensive nil-logger branch.
func TestGo_NilLoggerDoesNotPanic(t *testing.T) {
	done := make(chan struct{})
	panicguard.Go(nil, "unit", func() {
		defer close(done)
		panic("boom")
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("goroutine did not run")
	}
}
