// Copyright 2024-2026 The Chainloop Authors.
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
	"bytes"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttestationStatusPolicyEnforcementRows(t *testing.T) {
	const (
		strategyRowLabel = "Policy violation strategy"
		ignoredRowLabel  = "Ignored gate: false"
	)

	testCases := []struct {
		name        string
		status      *action.AttestationStatusResult
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:       "advisory strategy is hidden",
			status:     &action.AttestationStatusResult{},
			wantAbsent: []string{strategyRowLabel, action.PolicyViolationBlockingStrategyAdvisory, ignoredRowLabel},
		},
		{
			name: "enforced strategy names its source",
			status: &action.AttestationStatusResult{
				MustBlockOnPolicyViolations: true,
				PolicyEnforcementSource:     action.PolicyEnforcementSourceOrganization,
			},
			wantContain: []string{strategyRowLabel, "ENFORCED (by organization)"},
			wantAbsent:  []string{ignoredRowLabel},
		},
		{
			name: "ignored gate opt-outs are listed",
			status: &action.AttestationStatusResult{
				MustBlockOnPolicyViolations: true,
				PolicyEnforcementSource:     action.PolicyEnforcementSourceContract,
				IgnoredGateOptOuts:          []string{"sbom-banned-licenses", "slsa-checks"},
			},
			wantContain: []string{ignoredRowLabel, "sbom-banned-licenses, slsa-checks (enforcement cannot be relaxed)"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			tc.status.InitializedAt = &now
			tc.status.WorkflowMeta = &action.AttestationStatusWorkflowMeta{}
			tc.status.RunnerContext = &action.AttestationResultRunnerContext{}

			var buf bytes.Buffer
			require.NoError(t, fullStatusTableWithWriter(tc.status, &buf))
			got := buf.String()

			for _, want := range tc.wantContain {
				assert.Contains(t, got, want)
			}
			for _, absent := range tc.wantAbsent {
				assert.NotContains(t, got, absent)
			}
		})
	}
}

func TestVersionStringAttestation(t *testing.T) {
	testCases := []struct {
		name     string
		version  *action.ProjectVersion
		expected string
		pushed   bool
	}{
		{
			name: "empty version",
			version: &action.ProjectVersion{
				Version: "",
			},
			expected: "",
		},
		{
			name: "prerelease version to be released",
			version: &action.ProjectVersion{
				Version:        "1.0.0",
				Prerelease:     true,
				MarkAsReleased: true,
			},
			expected: "1.0.0 (will be released)",
		},
		{
			name: "already released version",
			version: &action.ProjectVersion{
				Version:    "1.0.0",
				Prerelease: false,
			},
			expected: "1.0.0 (already released)",
		},
		{
			name: "prerelease version",
			version: &action.ProjectVersion{
				Version:    "1.0.0-rc1",
				Prerelease: true,
			},
			expected: "1.0.0-rc1 (prerelease)",
		},
		{
			name: "prerelease version not marked for release",
			version: &action.ProjectVersion{
				Version:        "2.0.0-beta",
				Prerelease:     true,
				MarkAsReleased: false,
			},
			expected: "2.0.0-beta (prerelease)",
		},
		// Pushed state
		{
			name: "pushed - prerelease version",
			version: &action.ProjectVersion{
				Version:        "1.0.0-rc1",
				Prerelease:     true,
				MarkAsReleased: false,
			},
			pushed:   true,
			expected: "1.0.0-rc1 (prerelease)",
		},
		{
			name: "pushed - released version",
			version: &action.ProjectVersion{
				Version:        "1.0.0",
				Prerelease:     false,
				MarkAsReleased: true,
			},
			pushed:   true,
			expected: "1.0.0",
		},
		{
			name: "pushed - prerelease marked as released",
			version: &action.ProjectVersion{
				Version:        "2.0.0-beta",
				Prerelease:     true,
				MarkAsReleased: true,
			},
			pushed:   true,
			expected: "2.0.0-beta",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := versionStringAttestation(tc.version, tc.pushed)
			assert.Equal(t, tc.expected, result)
		})
	}
}
