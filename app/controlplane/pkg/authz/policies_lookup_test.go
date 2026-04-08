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

package authz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoliciesLookup(t *testing.T) {
	tests := []struct {
		name          string
		operation     string
		wantErr       bool
		wantErrIs     error
		wantActionIn  []string // at least one policy should have one of these actions
		wantPolicyLen int      // -1 means don't check
	}{
		{
			name:          "direct match - read operation",
			operation:     "/controlplane.v1.ReferrerService/DiscoverPrivate",
			wantPolicyLen: 1,
			wantActionIn:  []string{ActionRead},
		},
		{
			name:          "direct match - empty policies (open endpoint)",
			operation:     "/controlplane.v1.CASCredentialsService/Get",
			wantPolicyLen: 0,
		},
		{
			name:          "regex match - OrgMetricsService wildcard",
			operation:     "/controlplane.v1.OrgMetricsService/SomeMethod",
			wantPolicyLen: 1,
			wantActionIn:  []string{ActionList},
		},
		{
			name:      "unknown operation returns error",
			operation: "/controlplane.v1.NonExistentService/Unknown",
			wantErr:   true,
			wantErrIs: ErrOperationNotAllowed,
		},
		{
			name:      "empty operation returns error",
			operation: "",
			wantErr:   true,
			wantErrIs: ErrOperationNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policies, err := PoliciesLookup(tt.operation)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
				return
			}

			require.NoError(t, err)

			if tt.wantPolicyLen >= 0 {
				assert.Len(t, policies, tt.wantPolicyLen)
			}

			if len(tt.wantActionIn) > 0 && len(policies) > 0 {
				actions := make([]string, 0, len(policies))
				for _, p := range policies {
					actions = append(actions, p.Action)
				}
				assert.Subset(t, tt.wantActionIn, actions)
			}
		})
	}
}

func TestPoliciesLookupWriteOperation(t *testing.T) {
	// WorkflowService/Create should return a policy with action "create"
	policies, err := PoliciesLookup("/controlplane.v1.WorkflowService/Create")
	require.NoError(t, err)
	require.NotEmpty(t, policies)

	hasWriteAction := false
	for _, p := range policies {
		if p.Action == ActionCreate || p.Action == ActionUpdate || p.Action == ActionDelete {
			hasWriteAction = true
			break
		}
	}
	assert.True(t, hasWriteAction, "WorkflowService/Create should have a write action policy")
}
