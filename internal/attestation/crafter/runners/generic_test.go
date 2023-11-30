//
// Copyright 2023 The Chainloop Authors.
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

type genericSuite struct {
	suite.Suite
	runner *Generic
}

func (s *genericSuite) TestCheckEnv() {
	s.True(s.runner.CheckEnv())
}

func (s *genericSuite) TestListEnvVars() {
	s.Empty(s.runner.ListEnvVars())
}

func (s *genericSuite) TestResolveEnvVars() {
	result, errors := s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Equal(map[string]string{}, result)
}

func (s *genericSuite) TestRunURI() {
	s.Empty("", s.runner.RunURI())
}

func (s *genericSuite) TestRunnerName() {
	s.Equal("generic", s.runner.String())
}

// Run before each test
func (s *genericSuite) SetupTest() {
	s.runner = &Generic{}
}

// Run the tests
func TestGenericRunner(t *testing.T) {
	suite.Run(t, new(genericSuite))
}
