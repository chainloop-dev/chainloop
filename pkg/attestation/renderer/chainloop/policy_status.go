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

// PolicyStatus is the canonical, categorical policy outcome for an attestation.
// It collapses the raw enforcement/bypass/violation signals carried by
// PolicyEvaluationStatus into a single flat value so that list and describe
// surfaces can render a consistent badge without re-deriving.
type PolicyStatus string

const (
	PolicyStatusUnspecified   PolicyStatus = ""
	PolicyStatusNotApplicable PolicyStatus = "NOT_APPLICABLE"
	PolicyStatusPassed        PolicyStatus = "PASSED"
	PolicyStatusSkipped       PolicyStatus = "SKIPPED"
	PolicyStatusWarning       PolicyStatus = "WARNING"
	PolicyStatusBlocked       PolicyStatus = "BLOCKED"
	PolicyStatusBypassed      PolicyStatus = "BYPASSED"
)

// PolicyStatusSummary bundles the categorical status with the per-evaluation
// counters the UI needs to render a breakdown ("3/5 passed") without pulling
// the full attestation envelope.
type PolicyStatusSummary struct {
	Status   PolicyStatus
	Total    int
	Passed   int
	Skipped  int
	Violated int
}

// DerivePolicyStatusSummary is the single source of truth for computing the
// canonical PolicyStatus from the raw signals on PolicyEvaluationStatus.
//
// Rules (applied in order):
//
//	total == 0                                           -> NOT_APPLICABLE
//	(hasGatedViolations || strategy==ENFORCED) & bypass  -> BYPASSED
//	(hasGatedViolations || strategy==ENFORCED)           -> BLOCKED
//	hasViolations                                        -> WARNING
//	skipped > 0                                          -> SKIPPED
//	otherwise                                            -> PASSED
//
// The rule order matters: a bypassed gated violation resolves to BYPASSED,
// not BLOCKED; a warning (advisory violation) wins over a SKIPPED sibling.
func DerivePolicyStatusSummary(s *PolicyEvaluationStatus) PolicyStatusSummary {
	if s == nil || s.EvaluationsCount == 0 {
		return PolicyStatusSummary{Status: PolicyStatusNotApplicable}
	}

	summary := PolicyStatusSummary{
		Total:    s.EvaluationsCount,
		Passed:   s.PassedCount,
		Skipped:  s.SkippedCount,
		Violated: s.ViolationsCount,
	}

	enforced := s.HasGatedViolations || s.Strategy == PolicyViolationBlockingStrategyEnforced

	switch {
	case enforced && s.Bypassed:
		summary.Status = PolicyStatusBypassed
	case enforced && s.HasViolations:
		summary.Status = PolicyStatusBlocked
	case s.HasViolations:
		summary.Status = PolicyStatusWarning
	case s.SkippedCount > 0:
		summary.Status = PolicyStatusSkipped
	default:
		summary.Status = PolicyStatusPassed
	}

	return summary
}
