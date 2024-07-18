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

package biz

import (
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/metrics/prometheus/registry"

	"github.com/go-kratos/kratos/v2/log"
)

// PrometheusUseCase is a use case for Prometheus where some metrics are exposed
type PrometheusUseCase struct {
	logger *log.Helper
	// Use Cases
	orgUseCase        *OrganizationUseCase
	orgMetricsUseCase *OrgMetricsUseCase
	// Other
	registryManager *registry.ChainloopRegistryManager
}

// NewPrometheusUseCase creates a new PrometheusUseCase
func NewPrometheusUseCase(conf *conf.Bootstrap, orgUseCase *OrganizationUseCase, orgMetricsUseCase *OrgMetricsUseCase, logger log.Logger) *PrometheusUseCase {
	useCase := &PrometheusUseCase{
		orgUseCase:        orgUseCase,
		orgMetricsUseCase: orgMetricsUseCase,
		logger:            log.NewHelper(log.With(logger, "component", "biz/prometheus")),
	}

	registryManager := loadPrometheusRegistries(conf.PrometheusIntegration, orgMetricsUseCase, logger)
	useCase.registryManager = registryManager

	return useCase
}

// loadPrometheusRegistries loads the prometheus registries from the configuration
func loadPrometheusRegistries(conf []*conf.PrometheusIntegrationSpec, useCase *OrgMetricsUseCase, logger log.Logger) *registry.ChainloopRegistryManager {
	rm := registry.NewChainloopRegistryManager()

	for _, spec := range conf {
		reg := registry.NewPrometheusRegistry(spec.GetOrgName(), useCase, logger)
		rm.AddRegistry(reg)
	}

	return rm
}

// OrganizationHasRegistry checks if an organization has a registry
func (uc *PrometheusUseCase) OrganizationHasRegistry(orgName string) bool {
	return uc.registryManager.GetRegistryByName(orgName) != nil
}

// GetRegistryByOrganizationName returns a registry by organization name
func (uc *PrometheusUseCase) GetRegistryByOrganizationName(orgName string) *registry.PrometheusRegistry {
	return uc.registryManager.GetRegistryByName(orgName)
}
