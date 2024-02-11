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

package filesystem

import (
	"context"
	"fmt"
	"os"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/statemanager"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

func (s *testSuite) TestNew() {
	testCases := []struct {
		name      string
		statePath string
		wantErr   bool
	}{
		{
			name:      "empty state path",
			statePath: "",
			wantErr:   true,
		},
		{
			name:      "valid state path",
			statePath: "state.json",
			wantErr:   false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			_, err := New(tc.statePath)
			if tc.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *testSuite) TestWrite() {
	testCases := []struct {
		name    string
		state   *v1.CraftingState
		wantErr bool
	}{
		{
			name:    "empty state",
			wantErr: true,
		},
		{
			name:    "valid state",
			state:   s.exampleState,
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			sm, err := New(s.statePath)
			require.NoError(s.T(), err)

			err = sm.Write(context.Background(), "", tc.state)
			if tc.wantErr {
				s.Error(err)
				return
			}

			s.NoError(err)
			got := &v1.CraftingState{}
			err = sm.Read(context.Background(), "", got)
			s.NoError(err)
			s.Equal(tc.state, got)
			s.True(tc.state.DryRun)
		})
	}
}

func (s *testSuite) TestRead() {
	s.T().Run("empty input state", func(t *testing.T) {
		sm, err := New(s.statePath)
		require.NoError(t, err)
		err = sm.Read(context.Background(), "", nil)
		s.Error(err)
	})

	s.T().Run("no state found in path return NotFound error", func(t *testing.T) {
		sm, err := New(s.statePath)
		require.NoError(t, err)
		err = sm.Read(context.Background(), "", &v1.CraftingState{})
		s.Error(err)
		want := &statemanager.ErrNotFound{}
		s.ErrorAs(err, &want)
	})

	s.T().Run("we can read the state", func(t *testing.T) {
		sm, err := New("testdata/state.json")
		require.NoError(t, err)
		got := &v1.CraftingState{}
		err = sm.Read(context.Background(), "", got)
		require.NoError(s.T(), err)

		if ok := proto.Equal(s.exampleState, got); !ok {
			s.Fail(fmt.Sprintf("These two protobuf messages are not equal:\nexpected: %v\nactual:  %v", s.exampleState, got))
		}
	})
}

func (s *testSuite) TestReset() {
	s.T().Run("no state found in path return NotFound error", func(t *testing.T) {
		sm, err := New(s.statePath)
		require.NoError(t, err)
		err = sm.Reset(context.Background(), "")
		s.Error(err)
		want := &statemanager.ErrNotFound{}
		s.ErrorAs(err, &want)
	})

	s.T().Run("if state exists it can remove it", func(t *testing.T) {
		_, err := os.Create(s.statePath)
		require.NoError(s.T(), err)
		sm, err := New(s.statePath)
		require.NoError(t, err)
		err = sm.Reset(context.Background(), "")
		s.NoError(err)
	})
}
func (s *testSuite) TestInfo() {
	fs, _ := New("state.json")
	s.Equal("file://state.json", fs.Info(context.Background(), ""))
}

func (s *testSuite) TestInitialized() {
	s.T().Run("non existing", func(t *testing.T) {
		fs, err := New(s.statePath)
		require.NoError(s.T(), err)
		ok, err := fs.Initialized(context.Background(), "")
		require.NoError(s.T(), err)
		s.False(ok)
	})

	s.T().Run("already initialized", func(t *testing.T) {
		_, err := os.Create(s.statePath)
		require.NoError(s.T(), err)
		fs, err := New(s.statePath)
		require.NoError(s.T(), err)
		ok, err := fs.Initialized(context.Background(), "")
		require.NoError(s.T(), err)
		s.True(ok)
	})
}

type testSuite struct {
	suite.Suite
	statePath    string
	exampleState *v1.CraftingState
}

func (s *testSuite) SetupTest() {
	s.statePath = fmt.Sprintf("%s/attestation.json", s.T().TempDir())
	s.exampleState = &v1.CraftingState{DryRun: true, Attestation: &v1.Attestation{
		Annotations: map[string]string{"foo": "bar"},
	}}
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(testSuite))
}
