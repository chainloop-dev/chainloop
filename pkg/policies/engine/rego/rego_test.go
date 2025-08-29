//
// Copyright 2024-2025 The Chainloop Authors.
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

package rego

import (
	"context"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRego_VerifyWithValidPolicy(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/check_qa.rego")
	require.NoError(t, err)

	r := NewEngine()
	policy := &engine.Policy{
		Name:   "check approval",
		Source: regoContent,
	}

	t.Run("invalid input", func(t *testing.T) {
		result, err := r.Verify(context.TODO(), policy, []byte("{\"foo\": \"bar\"}"), nil)
		require.NoError(t, err)
		assert.False(t, result.Skipped)
		assert.Len(t, result.Violations, 2)
		assert.Contains(t, result.Violations, &engine.PolicyViolation{
			Subject:   policy.Name,
			Violation: "Container image is not approved",
		})
		assert.Contains(t, result.Violations, &engine.PolicyViolation{
			Subject:   policy.Name,
			Violation: "Container image is not released",
		})
	})

	t.Run("valid input", func(t *testing.T) {
		result, err := r.Verify(context.TODO(), policy, []byte(`
			{
				"kind": "CONTAINER_IMAGE", 
				"references": [{
					"metadata": {"name": "chainloop-platform-qa-approval"},
					"annotations": {"approval": "true"}
				}, {
					"metadata": {"name": "chainloop-platform-release-production"}
				}]
			}`), nil)
		require.NoError(t, err)
		assert.False(t, result.Skipped)
		assert.Len(t, result.Violations, 0)
	})
}

func TestRego_VerifyWithInputArray(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/arrays.rego")
	require.NoError(t, err)

	r := NewEngine()
	policy := &engine.Policy{
		Name:   "foobar",
		Source: regoContent,
	}

	t.Run("creates 'elements' field", func(t *testing.T) {
		result, err := r.Verify(context.TODO(), policy, []byte(`[{"foo": "bar"}, {"foo2":"bar2"}]`), nil)
		require.NoError(t, err)
		assert.Equal(t, "2", result.SkipReason)
	})
}

func TestRego_VerifyWithArguments(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/arguments.rego")
	require.NoError(t, err)

	r := NewEngine()
	policy := &engine.Policy{
		Name:   "foobar",
		Source: regoContent,
	}

	t.Run("no violations", func(t *testing.T) {
		result, err := r.Verify(context.TODO(), policy, []byte(`
			{
				"kind": "CONTAINER_IMAGE"
			}`),
			map[string]interface{}{"foo": "hello"},
		)
		require.NoError(t, err)
		assert.Len(t, result.Violations, 0)
	})

	t.Run("with violations", func(t *testing.T) {
		result, err := r.Verify(context.TODO(), policy, []byte(`
			{
				"kind": "CONTAINER_IMAGE"
			}`),
			map[string]interface{}{"foo": "bar"},
		)
		require.NoError(t, err)
		assert.Len(t, result.Violations, 1)
		assert.Contains(t, result.Violations, &engine.PolicyViolation{
			Subject: "foobar", Violation: "foo is bar"},
		)
	})
}
func TestRego_VerifyWithComplexArguments(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/arguments_array.rego")
	require.NoError(t, err)

	r := NewEngine()
	policy := &engine.Policy{
		Name:   "foobar",
		Source: regoContent,
	}

	t.Run("violation with array args", func(t *testing.T) {
		result, err := r.Verify(context.TODO(), policy, []byte(`
			{
				"kind": "CONTAINER_IMAGE"
			}`),
			map[string]interface{}{"foo": []string{"hello", "bar"}},
		)
		require.NoError(t, err)
		assert.Len(t, result.Violations, 1)
		assert.Contains(t, result.Violations, &engine.PolicyViolation{
			Subject: "foobar", Violation: "foo has bar"},
		)
	})

	t.Run("with array args", func(t *testing.T) {
		result, err := r.Verify(context.TODO(), policy, []byte(`
			{
				"kind": "CONTAINER_IMAGE"
			}`),
			map[string]interface{}{"foo": []string{"hello", "world"}},
		)
		require.NoError(t, err)
		assert.Len(t, result.Violations, 0)
	})
}

func TestRego_VerifyInvalidPolicy(t *testing.T) {
	// load policy without a default main rule
	regoContent, err := os.ReadFile("testfiles/policy_without_violations.rego")
	require.NoError(t, err)

	r := NewEngine()
	policy := &engine.Policy{
		Name:   "invalid",
		Source: regoContent,
	}

	t.Run("doesn't eval a main rule", func(t *testing.T) {
		_, err := r.Verify(context.TODO(), policy, []byte("{\"foo\": \"bar\"}"), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "neither 'result' nor 'violations' rule found")
	})
}

func TestRego_ResultFormat(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/result_format.rego")
	require.NoError(t, err)

	r := NewEngine()
	policy := &engine.Policy{
		Name:   "result-output",
		Source: regoContent,
	}

	t.Run("empty input", func(t *testing.T) {
		_, err := r.Verify(context.TODO(), policy, []byte{}, nil)
		assert.Error(t, err)
	})

	t.Run("invalid input", func(t *testing.T) {
		result, err := r.Verify(context.TODO(), policy, []byte("{\"foo\": \"bar\"}"), nil)
		require.NoError(t, err)
		assert.True(t, result.Skipped)
		assert.Equal(t, "invalid input", result.SkipReason)
		assert.False(t, result.Ignore)
	})

	t.Run("valid input, no violations", func(t *testing.T) {
		result, err := r.Verify(context.TODO(), policy, []byte("{\"specVersion\": \"1.5\"}"), nil)
		require.NoError(t, err)
		assert.False(t, result.Skipped)
		assert.Len(t, result.Violations, 0)
		assert.False(t, result.Ignore)
	})

	t.Run("valid input, violations", func(t *testing.T) {
		result, err := r.Verify(context.TODO(), policy, []byte("{\"specVersion\": \"1.4\"}"), nil)
		require.NoError(t, err)
		assert.False(t, result.Skipped)
		assert.Len(t, result.Violations, 1)
		assert.Equal(t, "wrong CycloneDX version. Expected 1.5, but it was 1.4", result.Violations[0].Violation)
		assert.False(t, result.Ignore)
	})

	t.Run("valid input, not ignore", func(t *testing.T) {
		result, err := r.Verify(context.TODO(), policy, []byte("{\"specVersion\": \"1.0\"}"), nil)
		require.NoError(t, err)
		assert.False(t, result.Skipped)
		assert.Len(t, result.Violations, 1)
		assert.Equal(t, "wrong CycloneDX version. Expected 1.5, but it was 1.0", result.Violations[0].Violation)
		assert.True(t, result.Ignore)
	})
}

func TestRego_ResultFormatWithoutIgnoreValue(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/result_format_without_ignore.rego")
	require.NoError(t, err)

	r := NewEngine()
	policy := &engine.Policy{
		Name:   "result-output",
		Source: regoContent,
	}

	t.Run("by default return always ignore to false", func(t *testing.T) {
		result, err := r.Verify(context.TODO(), policy, []byte("{\"foo\": \"bar\"}"), nil)
		require.NoError(t, err)
		assert.False(t, result.Ignore)
	})
}

func TestRego_WithRestrictiveMode(t *testing.T) {
	t.Run("forbidden functions", func(t *testing.T) {
		regoContent, err := os.ReadFile("testfiles/restrictive_mode.rego")
		require.NoError(t, err)

		r := NewEngine()
		policy := &engine.Policy{
			Name:   "policy",
			Source: regoContent,
		}

		_, err = r.Verify(context.TODO(), policy, []byte(`{}`), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rego_type_error: undefined function opa.runtime")
		assert.Contains(t, err.Error(), "rego_type_error: undefined function trace")
		assert.Contains(t, err.Error(), "rego_type_error: undefined function rego.parse_module")
	})

	t.Run("forbidden network requests", func(t *testing.T) {
		regoContent, err := os.ReadFile("testfiles/restrictive_mode_networking.rego")
		require.NoError(t, err)

		r := NewEngine()
		policy := &engine.Policy{
			Name:   "policy",
			Source: regoContent,
		}

		_, err = r.Verify(context.TODO(), policy, []byte(`{}`), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "eval_builtin_error: http.send: unallowed host: github.com")
	})

	t.Run("allowed network requests from defaults", func(t *testing.T) {
		regoContent, err := os.ReadFile("testfiles/restricted_mode_networking_allowed_host.rego")
		require.NoError(t, err)

		r := NewEngine()
		policy := &engine.Policy{
			Name:   "policy",
			Source: regoContent,
		}

		_, err = r.Verify(context.TODO(), policy, []byte(`{}`), nil)
		assert.NoError(t, err)
	})

	t.Run("allowed network requests from defaults plus custom domains", func(t *testing.T) {
		defaultHosts, err := os.ReadFile("testfiles/restricted_mode_networking_allowed_host.rego")
		require.NoError(t, err)
		customHosts, err := os.ReadFile("testfiles/restrictive_mode_networking.rego")
		require.NoError(t, err)

		r := NewEngine(WithAllowedNetworkDomains("github.com"))
		policy := &engine.Policy{
			Name:   "policy",
			Source: defaultHosts,
		}

		_, err = r.Verify(context.TODO(), policy, []byte(`{}`), nil)
		assert.NoError(t, err)

		policy = &engine.Policy{
			Name:   "policy",
			Source: customHosts,
		}

		_, err = r.Verify(context.TODO(), policy, []byte(`{}`), nil)
		assert.NoError(t, err)
	})
}

func TestRego_WithPermissiveMode(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/permissive_mode.rego")
	require.NoError(t, err)

	r := NewEngine(WithOperatingMode(EnvironmentModePermissive))
	policy := &engine.Policy{
		Name:   "policy",
		Source: regoContent,
	}

	t.Run("allowed functions", func(t *testing.T) {
		_, err = r.Verify(context.TODO(), policy, []byte(`{}`), nil)
		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "rego_type_error: undefined function opa.runtime")
		assert.NotContains(t, err.Error(), "rego_type_error: undefined function trace")
		assert.NotContains(t, err.Error(), "rego_type_error: undefined function rego.parse_module")
	})
}

func TestRego_MatchesParameters(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/matches_parameters.rego")
	require.NoError(t, err)

	r := NewEngine()
	policy := &engine.Policy{
		Name:   "matches-parameters-test",
		Source: regoContent,
	}

	t.Run("high severity matches medium expectation", func(t *testing.T) {
		matches, err := r.MatchesParameters(context.TODO(), policy,
			map[string]string{"severity": "high"},
			map[string]string{"severity": "medium"})
		require.NoError(t, err)
		assert.True(t, matches)
	})

	t.Run("low severity does not match high expectation", func(t *testing.T) {
		matches, err := r.MatchesParameters(context.TODO(), policy,
			map[string]string{"severity": "low"},
			map[string]string{"severity": "high"})
		require.NoError(t, err)
		assert.False(t, matches)
	})

	t.Run("critical severity matches critical expectation", func(t *testing.T) {
		matches, err := r.MatchesParameters(context.TODO(), policy,
			map[string]string{"severity": "critical"},
			map[string]string{"severity": "critical"})
		require.NoError(t, err)
		assert.True(t, matches)
	})

	t.Run("unknown severity parameter", func(t *testing.T) {
		matches, err := r.MatchesParameters(context.TODO(), policy,
			map[string]string{"severity": "unknown"},
			map[string]string{"severity": "medium"})
		require.NoError(t, err)
		assert.False(t, matches)
	})

	t.Run("empty parameters", func(t *testing.T) {
		matches, err := r.MatchesParameters(context.TODO(), policy,
			map[string]string{},
			map[string]string{})
		require.NoError(t, err)
		assert.False(t, matches)
	})
}

func TestRego_MatchesEvaluation(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/matches_evaluation.rego")
	require.NoError(t, err)

	r := NewEngine()
	policy := &engine.Policy{
		Name:   "matches-evaluation-test",
		Source: regoContent,
	}

	t.Run("evaluation with violations and high severity matches", func(t *testing.T) {
		violations := []string{"test violation"}
		evaluationParams := map[string]string{"severity": "high"}
		matches, err := r.MatchesEvaluation(context.TODO(), policy, violations, evaluationParams)
		require.NoError(t, err)
		assert.True(t, matches)
	})

	t.Run("evaluation without violations does not match", func(t *testing.T) {
		violations := []string{}
		evaluationParams := map[string]string{"severity": "high"}
		matches, err := r.MatchesEvaluation(context.TODO(), policy, violations, evaluationParams)
		require.NoError(t, err)
		assert.False(t, matches)
	})

	t.Run("evaluation with violations but wrong severity does not match", func(t *testing.T) {
		violations := []string{"test violation"}
		evaluationParams := map[string]string{"severity": "low"}
		matches, err := r.MatchesEvaluation(context.TODO(), policy, violations, evaluationParams)
		require.NoError(t, err)
		assert.False(t, matches)
	})

	t.Run("nil evaluation does not match", func(t *testing.T) {
		evaluationParams := map[string]string{"severity": "high"}
		matches, err := r.MatchesEvaluation(context.TODO(), policy, nil, evaluationParams)
		require.NoError(t, err)
		assert.False(t, matches)
	})

	t.Run("empty evaluation params", func(t *testing.T) {
		violations := []string{"test violation"}
		evaluationParams := map[string]string{}
		matches, err := r.MatchesEvaluation(context.TODO(), policy, violations, evaluationParams)
		require.NoError(t, err)
		assert.False(t, matches)
	})
}
