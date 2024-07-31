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

package prometheus

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/metrics/prometheus/registry"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *managerTestSuite) TestAddAndRetrieveRegistries() {
	r1 := registry.NewPrometheusRegistry("test1", nil, nil)
	r2 := registry.NewPrometheusRegistry("test2", nil, nil)
	s.manager.AddRegistry(r1)
	s.manager.AddRegistry(r2)
	s.Len(s.manager.registries, 2)

	s.Equal(r1, s.manager.GetRegistryByName("test1"))
	s.Equal(r2, s.manager.GetRegistryByName("test2"))
	s.Nil(s.manager.GetRegistryByName("test-not-found"))

	// delete one
	s.manager.DeleteRegistryByName("test1")
	s.Len(s.manager.registries, 1)
	s.Nil(s.manager.GetRegistryByName("test1"))
}

type managerTestSuite struct {
	suite.Suite
	manager *ChainloopRegistryManager
}

func (s *managerTestSuite) SetupTest() {
	s.manager = NewChainloopRegistryManager()
	require.NotNil(s.T(), s.manager)
	require.NotNil(s.T(), s.manager.registries)
}

func TestManager(t *testing.T) {
	suite.Run(t, new(managerTestSuite))
}
