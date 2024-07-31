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

package registry

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// This test looks obvious but checks an important property which is that the
// metrics must be different between two isolated registries, not a shared memory space.
func (s *registryTestSuite) TestIsolatedRegistries() {
	s.NotEqual(s.registry1, s.registry2)
	s.NotEqual(s.registry1.WorkflowRunDurationSeconds, s.registry2.WorkflowRunDurationSeconds)
}

func (s *registryTestSuite) TestName() {
	testCases := []struct {
		registry *PrometheusRegistry
		expected string
	}{
		{s.registry1, "test1"},
		{s.registry2, "test2"},
	}

	for _, tc := range testCases {
		s.Equal(tc.expected, tc.registry.Name)
	}
}

type registryTestSuite struct {
	suite.Suite
	registry1, registry2 *PrometheusRegistry
}

func (s *registryTestSuite) SetupTest() {
	s.registry1 = NewPrometheusRegistry("test1", nil, nil)
	s.registry2 = NewPrometheusRegistry("test2", nil, nil)
}

func TestRegistry(t *testing.T) {
	suite.Run(t, new(registryTestSuite))
}
