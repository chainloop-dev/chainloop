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
			name: "has_gates propagates from status",
			status: &PolicyEvaluationStatus{
				Strategy:         advisory,
				EvaluationsCount: 2,
				PassedCount:      2,
				HasGates:         true,
			},
			want: PolicyStatusSummary{
				Status:   PolicyStatusPassed,
				Total:    2,
				Passed:   2,
				HasGates: true,
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

// TestPredicateToPolicyStatusSummary exercises the attestation→summary chain
// used in the attestation-ingest path (biz.WorkflowRunUseCase.SaveAttestation):
// ProvenancePredicateV02.GetPolicyEvaluationStatus() → DerivePolicyStatusSummary.
// The two steps have their own unit tests but are never combined there, which
// is the exact translation the production code performs.
func TestPredicateToPolicyStatusSummary(t *testing.T) {
	testCases := []struct {
		name      string
		predicate *ProvenancePredicateV02
		want      PolicyStatusSummary
	}{
		{
			name:      "predicate with no policy evaluations => not applicable",
			predicate: &ProvenancePredicateV02{},
			want:      PolicyStatusSummary{Status: PolicyStatusNotApplicable},
		},
		{
			name: "stored counters => passed",
			predicate: &ProvenancePredicateV02{
				PolicyCheckBlockingStrategy: PolicyViolationBlockingStrategyAdvisory,
				PolicyEvaluationsCount:      3,
				PolicyPassedCount:           3,
			},
			want: PolicyStatusSummary{Status: PolicyStatusPassed, Total: 3, Passed: 3},
		},
		{
			name: "stored counters => skipped",
			predicate: &ProvenancePredicateV02{
				PolicyCheckBlockingStrategy: PolicyViolationBlockingStrategyAdvisory,
				PolicyEvaluationsCount:      3,
				PolicyPassedCount:           2,
				PolicySkippedCount:          1,
			},
			want: PolicyStatusSummary{Status: PolicyStatusSkipped, Total: 3, Passed: 2, Skipped: 1},
		},
		{
			name: "advisory violation => warning",
			predicate: &ProvenancePredicateV02{
				PolicyCheckBlockingStrategy: PolicyViolationBlockingStrategyAdvisory,
				PolicyEvaluationsCount:      2,
				PolicyPassedCount:           1,
				PolicyViolationsCount:       1,
				PolicyHasViolations:         true,
			},
			want: PolicyStatusSummary{Status: PolicyStatusWarning, Total: 2, Passed: 1, Violated: 1},
		},
		{
			name: "gated violation, not bypassed => blocked",
			predicate: &ProvenancePredicateV02{
				PolicyCheckBlockingStrategy: PolicyViolationBlockingStrategyAdvisory,
				PolicyEvaluationsCount:      2,
				PolicyPassedCount:           1,
				PolicyViolationsCount:       1,
				PolicyHasViolations:         true,
				PolicyHasGatedViolations:    true,
				PolicyAttBlocked:            true,
			},
			want: PolicyStatusSummary{Status: PolicyStatusBlocked, Total: 2, Passed: 1, Violated: 1},
		},
		{
			name: "gated violation bypassed => bypassed",
			predicate: &ProvenancePredicateV02{
				PolicyCheckBlockingStrategy: PolicyViolationBlockingStrategyAdvisory,
				PolicyEvaluationsCount:      2,
				PolicyPassedCount:           1,
				PolicyViolationsCount:       1,
				PolicyHasViolations:         true,
				PolicyHasGatedViolations:    true,
				PolicyBlockBypassEnabled:    true,
			},
			want: PolicyStatusSummary{Status: PolicyStatusBypassed, Total: 2, Passed: 1, Violated: 1},
		},
		{
			// Attestations signed before the skipped/passed counters existed
			// decode as zero. The predicate backfills from inline evaluations
			// before the summary is derived, so a skipped-only run must not
			// be misclassified as PASSED.
			name: "historic envelope with only inline skipped evaluations => skipped",
			predicate: &ProvenancePredicateV02{
				PolicyCheckBlockingStrategy: PolicyViolationBlockingStrategyAdvisory,
				PolicyEvaluationsCount:      2,
				PolicyEvaluations: map[string][]*PolicyEvaluation{
					"material-a": {{Skipped: true}, {Skipped: true}},
				},
			},
			want: PolicyStatusSummary{Status: PolicyStatusSkipped, Total: 2, Skipped: 2},
		},
		{
			// Historic envelopes can carry inline evaluations in the
			// fallback-shaped field; the backfill must read from there too.
			name: "historic envelope backfills from fallback evaluations => passed",
			predicate: &ProvenancePredicateV02{
				PolicyCheckBlockingStrategy: PolicyViolationBlockingStrategyAdvisory,
				PolicyEvaluationsCount:      2,
				PolicyEvaluationsFallback: map[string][]*PolicyEvaluation{
					"material-a": {{}, {}},
				},
			},
			want: PolicyStatusSummary{Status: PolicyStatusPassed, Total: 2, Passed: 2},
		},
		{
			name: "stored has_gates flag propagates to summary",
			predicate: &ProvenancePredicateV02{
				PolicyCheckBlockingStrategy: PolicyViolationBlockingStrategyAdvisory,
				PolicyEvaluationsCount:      2,
				PolicyPassedCount:           2,
				PolicyHasGates:              true,
			},
			want: PolicyStatusSummary{Status: PolicyStatusPassed, Total: 2, Passed: 2, HasGates: true},
		},
		{
			// Historic envelope with no stored has_gates flag: derive from
			// inline evaluations.
			name: "historic envelope derives has_gates from inline gate:true",
			predicate: &ProvenancePredicateV02{
				PolicyCheckBlockingStrategy: PolicyViolationBlockingStrategyAdvisory,
				PolicyEvaluationsCount:      2,
				PolicyPassedCount:           2,
				PolicyEvaluations: map[string][]*PolicyEvaluation{
					"material-a": {{Gate: true}, {}},
				},
			},
			want: PolicyStatusSummary{Status: PolicyStatusPassed, Total: 2, Passed: 2, HasGates: true},
		},
		{
			// Enforced strategy implies gating even without explicit gate:true
			// and even when the stored has_gates flag is missing (historic).
			name: "historic envelope with ENFORCED strategy derives has_gates=true",
			predicate: &ProvenancePredicateV02{
				PolicyCheckBlockingStrategy: PolicyViolationBlockingStrategyEnforced,
				PolicyEvaluationsCount:      1,
				PolicyPassedCount:           1,
			},
			want: PolicyStatusSummary{Status: PolicyStatusPassed, Total: 1, Passed: 1, HasGates: true},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := DerivePolicyStatusSummary(tc.predicate.GetPolicyEvaluationStatus())
			assert.Equal(t, tc.want, got)
		})
	}
}
