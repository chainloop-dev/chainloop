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

package testhelpers

import (
	"github.com/stretchr/testify/suite"
)

// Suite that creates a database and sets the schema before each test
type UseCasesEachTestSuite struct {
	suite.Suite
	*TestingUseCases
}

// Run only if the integration flag is set
func (s *UseCasesEachTestSuite) SetupSuite() {
	if !IntegrationTestsEnabled() {
		s.T().Skip()
	}
}

// Run before each test
func (s *UseCasesEachTestSuite) SetupTest() {
	s.TestingUseCases = NewTestingUseCases(s.T())
}

// Run after each test
func (s *UseCasesEachTestSuite) TearDownTest() {
	// s.TestingUseCases.DB.Close(s.T())
}
