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
package authz_test

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPolicies(t *testing.T) {
	testcases := []struct {
		name               string
		subject            *authz.SubjectAPIToken
		policies           []*authz.Policy
		wantErr            bool
		wantNumberPolicies int
	}{
		{
			name:    "empty policies and subject",
			wantErr: true,
		},
		{
			name: "no subject",
			policies: []*authz.Policy{
				authz.PolicyWorkflowContractList,
			},
			wantErr: true,
		},
		{
			name:    "no policies",
			subject: &authz.SubjectAPIToken{ID: uuid.NewString()},
			wantErr: true,
		},
		{
			name:    "adds two policies",
			subject: &authz.SubjectAPIToken{ID: uuid.NewString()},
			policies: []*authz.Policy{
				authz.PolicyWorkflowContractList,
				authz.PolicyReferrerRead,
			},
			wantNumberPolicies: 2,
		},
		{
			name: "handles duplicated policies",
			subject: &authz.SubjectAPIToken{
				ID: uuid.NewString(),
			},
			policies: []*authz.Policy{
				authz.PolicyWorkflowContractList,
				authz.PolicyWorkflowContractRead,
				authz.PolicyWorkflowContractUpdate,
				authz.PolicyWorkflowContractList,
				authz.PolicyArtifactDownload,
				authz.PolicyWorkflowContractUpdate,
				authz.PolicyArtifactDownload,
			},
			wantNumberPolicies: 4,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			enforcer, closer := testEnforcer(t)
			closer.Close()

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

			gotLength := enforcer.GetFilteredPolicy(0, tc.subject.String())
			assert.Len(t, gotLength, tc.wantNumberPolicies)
		})
	}
}

func TestAddPoliciesDuplication(t *testing.T) {
	want := []*authz.Policy{
		authz.PolicyWorkflowContractList,
		authz.PolicyWorkflowContractRead,
	}

	enforcer, closer := testEnforcer(t)
	defer closer.Close()
	sub := &authz.SubjectAPIToken{ID: uuid.NewString()}

	err := enforcer.AddPolicies(sub, want...)
	require.NoError(t, err)
	got := enforcer.GetFilteredPolicy(0, sub.String())
	assert.Len(t, got, 2)

	// Update the list of policies we want to add by appending an extra one
	want = append(want, authz.PolicyWorkflowContractUpdate)
	// AddPolicies only add the policies that are not already present preventing duplication
	err = enforcer.AddPolicies(sub, want...)
	assert.NoError(t, err)
	got = enforcer.GetFilteredPolicy(0, sub.String())
	assert.Len(t, got, 3)
}

func TestClearPolicies(t *testing.T) {
	want := []*authz.Policy{
		authz.PolicyWorkflowContractList,
		authz.PolicyWorkflowContractRead,
	}

	enforcer, closer := testEnforcer(t)
	defer closer.Close()
	sub := &authz.SubjectAPIToken{ID: uuid.NewString()}
	sub2 := &authz.SubjectAPIToken{ID: uuid.NewString()}

	// Create policies for two different subjects
	err := enforcer.AddPolicies(sub, want...)
	require.NoError(t, err)
	err = enforcer.AddPolicies(sub2, want...)
	require.NoError(t, err)
	// Each have 2 items
	got := enforcer.GetFilteredPolicy(0, sub.String())
	assert.Len(t, got, 2)

	// Clear all the policies for the subject
	err = enforcer.ClearPolicies(sub)
	assert.NoError(t, err)
	// there should be no policies left for this user
	got = enforcer.GetFilteredPolicy(0, sub.String())
	assert.Len(t, got, 0)
	// but the other user should still have 2
	got = enforcer.GetFilteredPolicy(0, sub2.String())
	assert.Len(t, got, 2)
}

func testEnforcer(t *testing.T) (*authz.Enforcer, io.Closer) {
	f, err := os.CreateTemp(t.TempDir(), "policy*.csv")
	if err != nil {
		require.FailNow(t, err.Error())
	}

	enforcer, err := authz.NewFiletypeEnforcer(f.Name())
	require.NoError(t, err)
	return enforcer, f
}

func TestMultiReplicaPropagation(t *testing.T) {
	// Create two enforcers that share the same database
	db := testhelpers.NewTestDatabase(t)
	defer db.Close(t)

	enforcerA, err := authz.NewDatabaseEnforcer(testhelpers.NewConfData(db, t).Database)
	require.NoError(t, err)
	enforcerB, err := authz.NewDatabaseEnforcer(testhelpers.NewConfData(db, t).Database)
	require.NoError(t, err)

	// Subject and policies to add
	sub := &authz.SubjectAPIToken{ID: uuid.NewString()}
	want := []*authz.Policy{authz.PolicyWorkflowContractList, authz.PolicyWorkflowContractRead}

	// Create policies in one enforcer
	err = enforcerA.AddPolicies(sub, want...)
	require.NoError(t, err)

	// Make sure it propagates to the other one
	got := enforcerA.GetFilteredPolicy(0, sub.String())
	assert.Len(t, got, 2)

	// it might take a bit for the policies to propagate to the other enforcer
	err = fnWithRetry(func() error {
		got = enforcerB.GetFilteredPolicy(0, sub.String())
		if len(got) == 2 {
			return nil
		}
		return fmt.Errorf("policies not propagated yet")
	})
	require.NoError(t, err)
	assert.Len(t, got, 2)

	// Then delete them from the second one and check propagation again
	require.NoError(t, enforcerB.ClearPolicies(sub))
	assert.Len(t, enforcerB.GetFilteredPolicy(0, sub.String()), 0)

	// Make sure it propagates to the other one
	err = fnWithRetry(func() error {
		got = enforcerA.GetFilteredPolicy(0, sub.String())
		if len(got) == 0 {
			return nil
		}

		return fmt.Errorf("policies not propagated yet")
	})
	require.NoError(t, err)
	assert.Len(t, enforcerA.GetFilteredPolicy(0, sub.String()), 0)
}

func fnWithRetry(f func() error) error {
	// Max 1 seconds
	return backoff.Retry(f, backoff.WithMaxRetries(backoff.NewConstantBackOff(100*time.Millisecond), 10))
}
