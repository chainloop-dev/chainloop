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

package policies

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine/rego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// radamsaMinIterationsPolicy mirrors the failing line of the real
// radamsa-min-iterations policy: to_number(input.args.min_iterations). With a
// multi-value (array) argument to_number raises an eval_type_error under the
// engine's strict-builtin-errors mode; with a scalar it evaluates and the
// numeric comparison runs.
const radamsaMinIterationsPolicy = `package main
import rego.v1

result := {
	"skipped": false,
	"violations": violations,
	"skip_reason": "",
}

violations contains msg if {
	n := to_number(input.args.min_iterations)
	n < 100
	msg := sprintf("min_iterations %v is below the required 100", [n])
}
`

// TestPFM6906ScalarOverrideFixesToNumberArrayError reproduces the exact failure
// reported in PFM-6906 and proves the new --policy-input override path fixes it,
// exercising the real rego engine end to end.
//
//   - Contract declares min_iterations=100 in its `with:`.
//   - The old --policy-input-from-file behavior APPENDS the runtime value, so
//     getInputArguments yields the array ["100","10"] and to_number fails with
//     "eval_type_error: to_number ... got array" — the reported error.
//   - The new --policy-input (and --policy-input-from-file-replace) behavior
//     REPLACES the value via OverrideRuntimeInputs, so min_iterations stays the
//     scalar "10", to_number succeeds, and the policy evaluates cleanly.
func TestPFM6906ScalarOverrideFixesToNumberArrayError(t *testing.T) {
	eng := rego.NewEngine()
	policy := &engine.Policy{Name: "radamsa-min-iterations", Source: []byte(radamsaMinIterationsPolicy)}
	material := []byte(`{}`)

	contractWith := map[string]string{"min_iterations": "100"}
	runtime := map[string]string{"min_iterations": "10"}

	t.Run("append path reproduces the to_number array error", func(t *testing.T) {
		with := MergeRuntimeInputs(contractWith, runtime)
		args := getInputArguments(with)

		// The append merge turns the scalar into a two-element list.
		require.Equal(t, []string{"100", "10"}, args["min_iterations"])

		_, err := eng.Verify(context.Background(), policy, material, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "to_number")
		assert.Contains(t, err.Error(), "array")
	})

	t.Run("override path keeps a scalar and evaluates cleanly", func(t *testing.T) {
		with := OverrideRuntimeInputs(contractWith, runtime)
		args := getInputArguments(with)

		// The override replaces the contract value; it stays a single scalar.
		require.Equal(t, "10", args["min_iterations"])

		res, err := eng.Verify(context.Background(), policy, material, args)
		require.NoError(t, err)
		require.Len(t, res.Violations, 1)
		assert.Contains(t, res.Violations[0].Violation, "min_iterations 10 is below the required 100")
	})
}
