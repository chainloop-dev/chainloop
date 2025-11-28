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

package wasm

import (
	"context"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/stretchr/testify/assert"
)

func TestNewEngine(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		eng := NewEngine()
		assert.NotNil(t, eng)
		assert.Equal(t, 5*time.Second, eng.executionTimeout)
		assert.False(t, eng.IncludeRawData)
		assert.False(t, eng.EnablePrint)
		// Base allowed hostnames should be included by default
		assert.Contains(t, eng.AllowedHostnames, "www.chainloop.dev")
		assert.Contains(t, eng.AllowedHostnames, "www.cisa.gov")
	})

	t.Run("with custom options", func(t *testing.T) {
		eng := NewEngine(
			engine.WithExecutionTimeout(30*time.Second),
			engine.WithIncludeRawData(true),
			engine.WithEnablePrint(true),
			engine.WithAllowedHostnames("api.example.com"),
		)
		assert.NotNil(t, eng)
		assert.Equal(t, 30*time.Second, eng.executionTimeout)
		assert.True(t, eng.IncludeRawData)
		assert.True(t, eng.EnablePrint)
		// Should include both custom and base hostnames
		assert.Contains(t, eng.AllowedHostnames, "api.example.com")
		assert.Contains(t, eng.AllowedHostnames, "www.chainloop.dev")
		assert.Contains(t, eng.AllowedHostnames, "www.cisa.gov")
	})
}

func TestEngineVerify_InvalidWASM(t *testing.T) {
	eng := NewEngine()

	ctx := context.Background()
	policy := &engine.Policy{
		Name:   "test-policy",
		Source: []byte("not valid wasm"),
	}
	input := []byte(`{"test": "data"}`)

	result, err := eng.Verify(ctx, policy, input, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create WASM plugin")
}

func TestEngineVerify_InvalidJSON(t *testing.T) {
	// This test would require a valid WASM module
	// Skipping for now as we need TinyGo to compile test WASM modules
	t.Skip("Requires compiled WASM test module")
}

func TestEngineImplementsPolicyEngine(_ *testing.T) {
	var _ engine.PolicyEngine = (*Engine)(nil)
}

func TestMatchesParameters(t *testing.T) {
	eng := NewEngine()

	ctx := context.Background()
	policy := &engine.Policy{
		Name:   "test-policy",
		Source: []byte{},
	}

	// Should always return true for WASM policies
	matches, err := eng.MatchesParameters(ctx, policy, map[string]string{"key": "value"}, map[string]string{"key": "value"})
	assert.NoError(t, err)
	assert.True(t, matches)
}

func TestMatchesEvaluation(t *testing.T) {
	eng := NewEngine()

	ctx := context.Background()
	policy := &engine.Policy{
		Name:   "test-policy",
		Source: []byte{},
	}

	// Should always return true for WASM policies
	matches, err := eng.MatchesEvaluation(ctx, policy, []string{"violation"}, map[string]string{"key": "value"})
	assert.NoError(t, err)
	assert.True(t, matches)
}
