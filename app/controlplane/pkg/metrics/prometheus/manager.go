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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/metrics/prometheus/registry"
)

type ChainloopRegistryManager struct {
	registries map[string]*registry.PrometheusRegistry
}

func NewChainloopRegistryManager() *ChainloopRegistryManager {
	return &ChainloopRegistryManager{
		registries: make(map[string]*registry.PrometheusRegistry),
	}
}

// AddRegistry adds a registry to the manager
func (rm *ChainloopRegistryManager) AddRegistry(reg *registry.PrometheusRegistry) {
	rm.registries[reg.Name] = reg
}

// GetRegistryByName returns a registry by name
func (rm *ChainloopRegistryManager) GetRegistryByName(name string) *registry.PrometheusRegistry {
	return rm.registries[name]
}

// DeleteRegistryByName deletes a registry by name
func (rm *ChainloopRegistryManager) DeleteRegistryByName(name string) {
	delete(rm.registries, name)
}
