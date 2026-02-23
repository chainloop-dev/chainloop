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

package cmd

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/stretchr/testify/require"
)

func TestValidatePolicyEnforcement(t *testing.T) {
	t.Run("does not block when strategy is advisory and policy is not gated", func(t *testing.T) {
		status := &action.AttestationStatusResult{
			PolicyEvaluations: map[string][]*action.PolicyEvaluation{
				"materials": {
					{
						Name: "cdx-fresh",
						Gate: false,
						Violations: []*action.PolicyViolation{
							{Message: "policy violation"},
						},
					},
				},
			},
			HasPolicyViolations:         true,
			MustBlockOnPolicyViolations: false,
		}

		err := validatePolicyEnforcement(status, false)
		require.NoError(t, err)
	})

	t.Run("returns gate error when strategy is advisory and policy is gated", func(t *testing.T) {
		status := &action.AttestationStatusResult{
			PolicyEvaluations: map[string][]*action.PolicyEvaluation{
				"materials": {
					{
						Name: "cdx-fresh",
						Gate: true,
						Violations: []*action.PolicyViolation{
							{Message: "policy violation"},
						},
					},
				},
			},
			HasPolicyViolations:         true,
			MustBlockOnPolicyViolations: false,
		}

		err := validatePolicyEnforcement(status, false)
		require.Error(t, err)
		var gateErr *GateError
		require.ErrorAs(t, err, &gateErr)
		require.Equal(t, "cdx-fresh", gateErr.PolicyName)
	})
}
