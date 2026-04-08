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

package crafter_test

import (
	"context"
	"errors"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	craftermocks "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// validCraftingState returns a minimal crafting state that satisfies LoadCraftingState requirements.
func validCraftingState() *api.CraftingState {
	return &api.CraftingState{
		Attestation: &api.Attestation{
			RunnerType: schemaapi.CraftingSchema_Runner_RUNNER_TYPE_UNSPECIFIED,
		},
	}
}

// setupReadExpectation configures the mock to populate state on Read calls.
func setupReadExpectation(sm *craftermocks.StateManager, digest string) *mock.Call {
	return sm.On("Read", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			st := args.Get(2).(*crafter.VersionedCraftingState)
			st.CraftingState = validCraftingState()
			st.UpdateCheckSum = digest
		}).Return(nil)
}

// newSuccessCollector creates a mock collector that succeeds.
// ID() is marked Maybe() since it's only called on failure paths.
func newSuccessCollector(t *testing.T, id string) *craftermocks.Collector {
	c := craftermocks.NewCollector(t)
	c.On("ID").Maybe().Return(id)
	c.On("Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return c
}

func TestRunCollectors(t *testing.T) {
	t.Run("always reloads state after collectors", func(t *testing.T) {
		sm := craftermocks.NewStateManager(t)
		setupReadExpectation(sm, "digest-1")
		sm.On("Info", mock.Anything, mock.Anything).Return("mock://run-1")

		// Collector modifies the digest (simulates a Write that updates UpdateCheckSum)
		c1 := craftermocks.NewCollector(t)
		c1.On("ID").Maybe().Return("c1")
		c1.On("Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				cr := args.Get(1).(*crafter.Crafter)
				cr.CraftingState.UpdateCheckSum = "digest-after-write"
			}).Return(nil)

		cr, err := crafter.NewCrafter(sm, nil)
		require.NoError(t, err)
		cr.RegisterCollectors(c1)

		cr.RunCollectors(context.Background(), "run-1", nil)

		// Read before collectors + unconditional Read after collectors
		sm.AssertNumberOfCalls(t, "Read", 2)
		c1.AssertCalled(t, "Collect", mock.Anything, mock.Anything, "run-1", mock.Anything)
	})

	t.Run("collector failure does not stop other collectors", func(t *testing.T) {
		sm := craftermocks.NewStateManager(t)
		setupReadExpectation(sm, "d")
		sm.On("Info", mock.Anything, mock.Anything).Return("mock://run-1")

		c1 := craftermocks.NewCollector(t)
		c1.On("ID").Return("failing")
		c1.On("Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("collector error"))

		c2 := newSuccessCollector(t, "ok")

		cr, err := crafter.NewCrafter(sm, nil)
		require.NoError(t, err)
		cr.RegisterCollectors(c1, c2)

		cr.RunCollectors(context.Background(), "run-1", nil)

		c1.AssertCalled(t, "Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		c2.AssertCalled(t, "Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		sm.AssertNumberOfCalls(t, "Read", 2)
	})

	t.Run("reloads state even with no collectors", func(t *testing.T) {
		sm := craftermocks.NewStateManager(t)
		setupReadExpectation(sm, "d")
		sm.On("Info", mock.Anything, mock.Anything).Return("mock://run-1")

		cr, err := crafter.NewCrafter(sm, nil)
		require.NoError(t, err)

		cr.RunCollectors(context.Background(), "run-1", nil)

		// Read before + unconditional Read after, even with no collectors
		sm.AssertNumberOfCalls(t, "Read", 2)
	})

	t.Run("reloads state when digest is stale (old server)", func(t *testing.T) {
		sm := craftermocks.NewStateManager(t)
		setupReadExpectation(sm, "digest-1")
		sm.On("Info", mock.Anything, mock.Anything).Return("mock://run-1")

		// Collector persists state via Write but the old server doesn't return
		// a new digest, so UpdateCheckSum stays unchanged (stale).
		c1 := craftermocks.NewCollector(t)
		c1.On("ID").Maybe().Return("c1")
		c1.On("Collect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		cr, err := crafter.NewCrafter(sm, nil)
		require.NoError(t, err)
		cr.RegisterCollectors(c1)

		cr.RunCollectors(context.Background(), "run-1", nil)

		// Unconditional reload ensures the stale digest is refreshed
		sm.AssertNumberOfCalls(t, "Read", 2)
		c1.AssertCalled(t, "Collect", mock.Anything, mock.Anything, "run-1", mock.Anything)
	})

	t.Run("state load failure aborts before running collectors", func(t *testing.T) {
		sm := craftermocks.NewStateManager(t)
		sm.On("Read", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("read error"))
		sm.On("Info", mock.Anything, mock.Anything).Return("mock://run-1")

		c1 := craftermocks.NewCollector(t)

		cr, err := crafter.NewCrafter(sm, nil)
		require.NoError(t, err)
		cr.RegisterCollectors(c1)

		cr.RunCollectors(context.Background(), "run-1", nil)

		// Collector should never be called
		c1.AssertNotCalled(t, "Collect")
	})
}
