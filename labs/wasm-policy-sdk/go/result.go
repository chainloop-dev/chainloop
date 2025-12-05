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

package _go

import "fmt"

// Result represents a policy execution result.
// This type is designed to work with pdk.OutputJSON() - all fields are simple types.
type Result struct {
	Skipped    bool     `json:"skipped"`
	Violations []string `json:"violations"`
	SkipReason string   `json:"skip_reason"`
	Ignore     bool     `json:"ignore"`
}

// Success creates a successful result (no violations, not skipped).
func Success() Result {
	return Result{
		Skipped:    false,
		Violations: []string{},
		SkipReason: "",
		Ignore:     false,
	}
}

// Fail creates a failed result with one or more violations.
func Fail(violations ...string) Result {
	return Result{
		Skipped:    false,
		Violations: violations,
		SkipReason: "",
		Ignore:     false,
	}
}

// Skip creates a skipped result with a reason.
// Use this when the policy doesn't apply to the material.
func Skip(reason string) Result {
	return Result{
		Skipped:    true,
		Violations: []string{},
		SkipReason: reason,
		Ignore:     false,
	}
}

// Skipf creates a skipped result with a formatted reason.
func Skipf(format string, args ...interface{}) Result {
	return Result{
		Skipped:    true,
		Violations: []string{},
		SkipReason: fmt.Sprintf(format, args...),
		Ignore:     false,
	}
}

// AddViolation adds a violation message to the result.
func (r *Result) AddViolation(msg string) {
	r.Violations = append(r.Violations, msg)
}

// AddViolationf adds a formatted violation message to the result.
func (r *Result) AddViolationf(format string, args ...interface{}) {
	r.Violations = append(r.Violations, fmt.Sprintf(format, args...))
}

// HasViolations returns true if there are any violations.
func (r *Result) HasViolations() bool {
	return len(r.Violations) > 0
}

// IsSuccess returns true if the policy passed (no violations, not skipped).
func (r *Result) IsSuccess() bool {
	return !r.Skipped && len(r.Violations) == 0
}
