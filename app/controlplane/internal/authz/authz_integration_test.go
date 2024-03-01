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

package authz_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
