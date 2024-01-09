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

// Authorization package
package authz

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPolicies(t *testing.T) {
	testcases := []struct {
		name     string
		subject  *SubjectAPIToken
		policies []*Policy
		wantErr  bool
	}{
		{
			name:    "empty policies",
			wantErr: true,
		},
		{
			name: "no subject",
			policies: []*Policy{
				PolicyWorkflowContractList,
			},
			wantErr: true,
		},
		{
			name: "happy path",
			subject: &SubjectAPIToken{
				ID: uuid.NewString(),
			},
			policies: []*Policy{
				PolicyWorkflowContractList,
				PolicyReferrerRead,
			},
		},
		{
			name: "another happy path",
			subject: &SubjectAPIToken{
				ID: uuid.NewString(),
			},
			policies: []*Policy{
				PolicyWorkflowContractList,
				PolicyWorkflowContractRead,
				PolicyWorkflowContractUpdate,
				PolicyArtifactDownload,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			enforcer, closer := testEnforcer(t)
			defer closer.Close()

			err := enforcer.AddPolicies(tc.subject, tc.policies...)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			for _, p := range tc.policies {
				ok := enforcer.HasPolicy(tc.subject.String(), p.Resource, p.Action)
				assert.True(t, ok, fmt.Sprintf("policy %s:%s not found", p.Resource, p.Action))
			}
		})
	}
}

func testEnforcer(t *testing.T) (*Enforcer, io.Closer) {
	policyFilepath := filepath.Join(t.TempDir(), "policy.csv")
	// create the file if it doesn't exist
	f, err := os.Create(policyFilepath)
	if err != nil {
		require.FailNow(t, err.Error())
	}

	enforcer, err := NewFiletypeEnforcer(policyFilepath)
	require.NoError(t, err)
	return enforcer, f
}
