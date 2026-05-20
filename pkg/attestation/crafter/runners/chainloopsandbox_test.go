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

package runners

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type chainloopSandboxSuite struct {
	suite.Suite
	runner *ChainloopSandbox
}

func (s *chainloopSandboxSuite) TestCheckEnv() {
	testCases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			name: "env var not set",
			env:  map[string]string{},
			want: false,
		},
		{
			name: "env var set to non-empty value",
			env:  map[string]string{"CHAINLOOP_SANDBOX": "1"},
			want: true,
		},
		{
			name: "env var set to empty string",
			env:  map[string]string{"CHAINLOOP_SANDBOX": ""},
			want: false,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			t.Setenv("CHAINLOOP_SANDBOX", "")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			s.Equal(tc.want, s.runner.CheckEnv())
		})
	}
}

func (s *chainloopSandboxSuite) TestListEnvVars() {
	s.Empty(s.runner.ListEnvVars())
}

func (s *chainloopSandboxSuite) TestResolveEnvVars() {
	resolved, errors := s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Empty(resolved)
}

func (s *chainloopSandboxSuite) TestRunURI() {
	s.Empty(s.runner.RunURI())
}

func (s *chainloopSandboxSuite) TestWorkflowFilePath() {
	s.Empty(s.runner.WorkflowFilePath())
}

func (s *chainloopSandboxSuite) TestIsAuthenticated() {
	s.False(s.runner.IsAuthenticated())
}

func (s *chainloopSandboxSuite) TestEnvironment() {
	s.Equal(Unknown, s.runner.Environment())
}

func (s *chainloopSandboxSuite) TestRunnerName() {
	s.Equal("CHAINLOOP_SANDBOX", s.runner.ID().String())
}

func (s *chainloopSandboxSuite) SetupTest() {
	s.runner = NewChainloopSandbox()
}

func TestChainloopSandboxRunner(t *testing.T) {
	suite.Run(t, new(chainloopSandboxSuite))
}
