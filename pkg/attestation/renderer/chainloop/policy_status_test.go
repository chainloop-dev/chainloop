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

package chainloop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDerivePolicyStatusSummary(t *testing.T) {
	enforced := PolicyViolationBlockingStrategyEnforced
	advisory := PolicyViolationBlockingStrategyAdvisory

	testCases := []struct {
		name   string
		status *PolicyEvaluationStatus
		want   PolicyStatusSummary
	}{
		{
			name:   "nil input is not applicable",
			status: nil,
			want:   PolicyStatusSummary{Status: PolicyStatusNotApplicable},
		},
		{
			name: "no evaluations is not applicable",
			status: &PolicyEvaluationStatus{
				Strategy:         advisory,
				EvaluationsCount: 0,
			},
			want: PolicyStatusSummary{Status: PolicyStatusNotApplicable},
		},
		{
			name: "all evaluations pass",
			status: &PolicyEvaluationStatus{
				Strategy:         advisory,
				EvaluationsCount: 3,
				PassedCount:      3,
			},
			want: PolicyStatusSummary{
				Status: PolicyStatusPassed,
				Total:  3,
				Passed: 3,
			},
		},
		{
			name: "some skipped, none violated => skipped",
			status: &PolicyEvaluationStatus{
				Strategy:         advisory,
				EvaluationsCount: 3,
				PassedCount:      2,
				SkippedCount:     1,
			},
			want: PolicyStatusSummary{
				Status:  PolicyStatusSkipped,
				Total:   3,
				Passed:  2,
				Skipped: 1,
			},
		},
		{
			name: "all skipped, none violated => skipped",
			status: &PolicyEvaluationStatus{
				Strategy:         advisory,
				EvaluationsCount: 2,
				SkippedCount:     2,
			},
			want: PolicyStatusSummary{
				Status:  PolicyStatusSkipped,
				Total:   2,
				Skipped: 2,
			},
		},
		{
			name: "advisory violation with no gating => warning",
			status: &PolicyEvaluationStatus{
				Strategy:         advisory,
				EvaluationsCount: 2,
				PassedCount:      1,
				ViolationsCount:  3,
				HasViolations:    true,
			},
			want: PolicyStatusSummary{
				Status:   PolicyStatusWarning,
				Total:    2,
				Passed:   1,
				Violated: 3,
			},
		},
		{
			name: "gated violation, not bypassed => blocked",
			status: &PolicyEvaluationStatus{
				Strategy:           advisory,
				EvaluationsCount:   2,
				PassedCount:        1,
				ViolationsCount:    1,
				HasViolations:      true,
				HasGatedViolations: true,
				Blocked:            true,
			},
			want: PolicyStatusSummary{
				Status:   PolicyStatusBlocked,
				Total:    2,
				Passed:   1,
				Violated: 1,
			},
		},
		{
			name: "enforced strategy with violations, not bypassed => blocked",
			status: &PolicyEvaluationStatus{
				Strategy:         enforced,
				EvaluationsCount: 2,
				PassedCount:      1,
				ViolationsCount:  2,
				HasViolations:    true,
				Blocked:          true,
			},
			want: PolicyStatusSummary{
				Status:   PolicyStatusBlocked,
				Total:    2,
				Passed:   1,
				Violated: 2,
			},
		},
		{
			name: "gated violation bypassed => bypassed",
			status: &PolicyEvaluationStatus{
				Strategy:           advisory,
				EvaluationsCount:   2,
				PassedCount:        1,
				ViolationsCount:    1,
				HasViolations:      true,
				HasGatedViolations: true,
				Bypassed:           true,
			},
			want: PolicyStatusSummary{
				Status:   PolicyStatusBypassed,
				Total:    2,
				Passed:   1,
				Violated: 1,
			},
		},
		{
			name: "enforced strategy with violations bypassed => bypassed",
			status: &PolicyEvaluationStatus{
				Strategy:         enforced,
				EvaluationsCount: 1,
				ViolationsCount:  1,
				HasViolations:    true,
				Bypassed:         true,
			},
			want: PolicyStatusSummary{
				Status:   PolicyStatusBypassed,
				Total:    1,
				Violated: 1,
			},
		},
		{
			name: "enforced but no violations yet => passed",
			status: &PolicyEvaluationStatus{
				Strategy:         enforced,
				EvaluationsCount: 2,
				PassedCount:      2,
			},
			want: PolicyStatusSummary{
				Status: PolicyStatusPassed,
				Total:  2,
				Passed: 2,
			},
		},
		{
			name: "warning + skipped mix => warning (violations dominate skips)",
			status: &PolicyEvaluationStatus{
				Strategy:         advisory,
				EvaluationsCount: 3,
				PassedCount:      1,
				SkippedCount:     1,
				ViolationsCount:  1,
				HasViolations:    true,
			},
			want: PolicyStatusSummary{
				Status:   PolicyStatusWarning,
				Total:    3,
				Passed:   1,
				Skipped:  1,
				Violated: 1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := DerivePolicyStatusSummary(tc.status)
			assert.Equal(t, tc.want, got)
		})
	}
}
