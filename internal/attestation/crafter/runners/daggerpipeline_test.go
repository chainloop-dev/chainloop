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

package runners

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type daggerPipelineSuite struct {
	suite.Suite
	runner *DaggerPipeline
}

func (s *daggerPipelineSuite) TestCheckEnv() {
	testCases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			name: "empty",
			env:  map[string]string{},
			want: false,
		},
		{
			name: "present",
			env: map[string]string{
				"CHAINLOOP_DAGGER_CLIENT": "v1.0.0",
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			os.Unsetenv("CHAINLOOP_DAGGER_CLIENT")

			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			s.Equal(tc.want, s.runner.CheckEnv())
		})
	}
}

func (s *daggerPipelineSuite) TestListEnvVars() {
	assert.Equal(s.T(), []*EnvVarDefinition{{"CHAINLOOP_DAGGER_CLIENT", false}}, s.runner.ListEnvVars())
}

func (s *daggerPipelineSuite) TestResolveEnvVars() {
	resolvedEnvVars, errors := s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Equal(map[string]string{"CHAINLOOP_DAGGER_CLIENT": "v0.6.0"}, resolvedEnvVars)
}

func (s *daggerPipelineSuite) TestRunURI() {
	s.Equal("", s.runner.RunURI())
}

func (s *daggerPipelineSuite) TestRunnerName() {
	s.Equal("DAGGER_PIPELINE", s.runner.ID().String())
}

// Run before each test
func (s *daggerPipelineSuite) SetupTest() {
	s.runner = NewDaggerPipeline()
	t := s.T()
	t.Setenv("CHAINLOOP_DAGGER_CLIENT", "v0.6.0")
}

// Run the tests
func TestDaggerPipelineRunner(t *testing.T) {
	suite.Run(t, new(daggerPipelineSuite))
}
