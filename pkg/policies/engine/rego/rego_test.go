//
// Copyright 2024 The Chainloop Authors.
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

	r := &Rego{}
	policy := &engine.Policy{
		Name:   "check approval",
		Source: regoContent,
	}

	t.Run("invalid input", func(t *testing.T) {
		violations, err := r.Verify(context.TODO(), policy, []byte("{\"foo\": \"bar\"}"), nil)
		require.NoError(t, err)
		assert.Len(t, violations, 2)
		assert.Contains(t, violations, &engine.PolicyViolation{
			Subject:   policy.Name,
			Violation: "Container image is not approved",
		})
		assert.Contains(t, violations, &engine.PolicyViolation{
			Subject:   policy.Name,
			Violation: "Container image is not released",
		})
	})

	t.Run("valid input", func(t *testing.T) {
		violations, err := r.Verify(context.TODO(), policy, []byte(`
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
		assert.Len(t, violations, 0)
	})
}

func TestRego_VerifyWithDeprecatedPolicy(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/check_qa_deprecated.rego")
	require.NoError(t, err)

	r := &Rego{}
	policy := &engine.Policy{
		Name:   "check approval",
		Source: regoContent,
	}

	t.Run("invalid input", func(t *testing.T) {
		violations, err := r.Verify(context.TODO(), policy, []byte("{\"foo\": \"bar\"}"), nil)
		require.NoError(t, err)
		assert.Len(t, violations, 2)
		assert.Contains(t, violations, &engine.PolicyViolation{
			Subject:   policy.Name,
			Violation: "Container image is not approved",
		})
		assert.Contains(t, violations, &engine.PolicyViolation{
			Subject:   policy.Name,
			Violation: "Container image is not released",
		})
	})

	t.Run("valid input", func(t *testing.T) {
		violations, err := r.Verify(context.TODO(), policy, []byte(`
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
		assert.Len(t, violations, 0)
	})
}

func TestRego_VerifyWithArguments(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/arguments.rego")
	require.NoError(t, err)

	r := &Rego{}
	policy := &engine.Policy{
		Name:   "foobar",
		Source: regoContent,
	}

	t.Run("no violations", func(t *testing.T) {
		violations, err := r.Verify(context.TODO(), policy, []byte(`
			{
				"kind": "CONTAINER_IMAGE"
			}`),
			map[string]interface{}{"foo": "hello"},
		)
		require.NoError(t, err)
		assert.Len(t, violations, 0)
	})

	t.Run("with violations", func(t *testing.T) {
		violations, err := r.Verify(context.TODO(), policy, []byte(`
			{
				"kind": "CONTAINER_IMAGE"
			}`),
			map[string]interface{}{"foo": "bar"},
		)
		require.NoError(t, err)
		assert.Len(t, violations, 1)
		assert.Contains(t, violations, &engine.PolicyViolation{
			Subject: "foobar", Violation: "foo is bar"},
		)
	})
}
func TestRego_VerifyWithComplexArguments(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/arguments_array.rego")
	require.NoError(t, err)

	r := &Rego{}
	policy := &engine.Policy{
		Name:   "foobar",
		Source: regoContent,
	}

	t.Run("violation with array args", func(t *testing.T) {
		violations, err := r.Verify(context.TODO(), policy, []byte(`
			{
				"kind": "CONTAINER_IMAGE"
			}`),
			map[string]interface{}{"foo": []string{"hello", "bar"}},
		)
		require.NoError(t, err)
		assert.Len(t, violations, 1)
		assert.Contains(t, violations, &engine.PolicyViolation{
			Subject: "foobar", Violation: "foo has bar"},
		)
	})

	t.Run("with array args", func(t *testing.T) {
		violations, err := r.Verify(context.TODO(), policy, []byte(`
			{
				"kind": "CONTAINER_IMAGE"
			}`),
			map[string]interface{}{"foo": []string{"hello", "world"}},
		)
		require.NoError(t, err)
		assert.Len(t, violations, 0)
	})
}

func TestRego_VerifyInvalidPolicy(t *testing.T) {
	// load policy without a default main rule
	regoContent, err := os.ReadFile("testfiles/policy_without_violations.rego")
	require.NoError(t, err)

	r := &Rego{}
	policy := &engine.Policy{
		Name:   "invalid",
		Source: regoContent,
	}

	t.Run("doesn't eval a main rule", func(t *testing.T) {
		violations, err := r.Verify(context.TODO(), policy, []byte("{\"foo\": \"bar\"}"), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no 'violations' nor 'deny' rule found")
		assert.Len(t, violations, 0)
	})
}

func TestRego_WithForbiddenBuiltInFunctions(t *testing.T) {
	regoContent, err := os.ReadFile("testfiles/forbidden_functions.rego")
	require.NoError(t, err)

	r := &Rego{}
	policy := &engine.Policy{
		Name:   "policy",
		Source: regoContent,
	}

	t.Run("forbidden functions", func(t *testing.T) {
		violations, err := r.Verify(context.TODO(), policy, []byte(`{}`), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rego_type_error: undefined function opa.runtime")
		assert.Contains(t, err.Error(), "rego_type_error: undefined function trace")
		assert.Contains(t, err.Error(), "rego_type_error: undefined function rego.parse_module")
		assert.Len(t, violations, 0)
	})
}
