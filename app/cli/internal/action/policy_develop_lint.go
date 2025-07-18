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

package action

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/chainloop-dev/chainloop/app/cli/internal/policydevel"
)

type PolicyLintOpts struct {
	PolicyPath string
	Format     bool
}

type PolicyLintResult struct {
	Valid  bool
	Errors []string
}

type PolicyLint struct {
	*ActionsOpts
}

func NewPolicyLint(actionOpts *ActionsOpts) (*PolicyLint, error) {
	return &PolicyLint{
		ActionsOpts: actionOpts,
	}, nil
}

func (action *PolicyLint) Run(_ context.Context, opts *PolicyLintOpts) (*PolicyLintResult, error) {
	// Resolve absolute path to policy directory
	absPath, err := filepath.Abs(opts.PolicyPath)
	if err != nil {
		return nil, fmt.Errorf("resolving absolute path: %w", err)
	}

	// Read policies
	policy, err := policydevel.Lookup(absPath, opts.Format)
	if err != nil {
		return nil, fmt.Errorf("loading policy: %w", err)
	}

	// Run all validations
	policy.Validate()

	// Prepare result
	result := &PolicyLintResult{
		Valid:  !policy.HasErrors(),
		Errors: make([]string, 0, len(policy.Errors)),
	}

	// Convert validation errors to strings
	for _, err := range policy.Errors {
		result.Errors = append(result.Errors, err.Error())
	}

	return result, nil
}
