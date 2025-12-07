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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestSimpleWASMExecution verifies that basic WASM policy execution works
func TestSimpleWASMExecution(t *testing.T) {
	// Load a simple test WASM policy
	wasmPath := filepath.Join("testdata", "simple_test_policy.wasm")
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err, "Failed to load simple test WASM policy")

	eng := NewEngine()
	ctx := context.Background()

	policy := &engine.Policy{
		Name:   "simple-test",
		Source: wasmBytes,
	}
	input := []byte(`{}`)

	result, err := eng.Verify(ctx, policy, input, nil)
	require.NoError(t, err, "Policy execution should not error")
	require.NotNil(t, result)

	// Should have one violation: "test violation"
	assert.Len(t, result.Violations, 1)
	assert.Equal(t, "test violation", result.Violations[0].Violation)
}

// TestFilesystemIsolation verifies that WASM policies CANNOT access the host filesystem
//
// IMPORTANT SECURITY VERIFICATION:
// This test confirms that the Extism runtime provides filesystem isolation by default
// when EnableWasi is true. Even without explicit AllowedPaths configuration, WASM policies
// are sandboxed and cannot access sensitive host filesystem paths like:
// - /etc/passwd, /etc/hosts (system files)
// - / (root directory)
// - . (current working directory)
//
// The test policy attempts to stat() these paths and reports violations if successful.
// A passing test (no violations) means filesystem isolation is working correctly.
func TestFilesystemIsolation(t *testing.T) {
	// Load the compiled test WASM policy that attempts filesystem access
	wasmPath := filepath.Join("testdata", "filesystem_test_policy.wasm")
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err, "Failed to load test WASM policy - run 'make build-test-wasm' first")

	eng := NewEngine()
	ctx := context.Background()

	policy := &engine.Policy{
		Name:   "filesystem-security-test",
		Source: wasmBytes,
	}
	input := []byte(`{}`) // Empty input, policy will try filesystem access

	t.Run("verify filesystem isolation is working", func(t *testing.T) {
		result, err := eng.Verify(ctx, policy, input, nil)
		require.NoError(t, err, "Policy execution should not error")
		require.NotNil(t, result)

		// Check if the policy reported any security violations
		// If there are violations, it means the policy was able to access host files (BAD)
		// If there are no violations, it means filesystem access was blocked (GOOD)
		if len(result.Violations) > 0 {
			t.Logf("SECURITY WARNING: Policy accessed host filesystem!")
			for _, violation := range result.Violations {
				t.Logf("  - %s", violation.Violation)
			}
			require.FailNow(t, "SECURITY ISSUE: WASM policy was able to access host filesystem without isolation")
		} else {
			t.Log("Filesystem isolation is working correctly - policy was blocked from accessing host files")
			t.Log("Verified: /etc/passwd, /etc/hosts, current directory, and root directory are all inaccessible")
		}
	})
}
