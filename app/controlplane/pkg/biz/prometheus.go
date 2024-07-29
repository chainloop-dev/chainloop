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
	"context"
	"time"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	chainloopprometheus "github.com/chainloop-dev/chainloop/app/controlplane/pkg/metrics/prometheus"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/metrics/prometheus/registry"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-kratos/kratos/v2/log"
)

// PrometheusUseCase is a use case for Prometheus where some metrics are exposed
type PrometheusUseCase struct {
	logger *log.Helper
	// Use Cases
	orgUseCase        *OrganizationUseCase
	orgMetricsUseCase *OrgMetricsUseCase
	// Other
	registryManager *chainloopprometheus.ChainloopRegistryManager
}

// NewPrometheusUseCase creates a new PrometheusUseCase
func NewPrometheusUseCase(conf []*conf.PrometheusIntegrationSpec, orgUseCase *OrganizationUseCase, orgMetricsUseCase *OrgMetricsUseCase, logger log.Logger) *PrometheusUseCase {
	useCase := &PrometheusUseCase{
		orgUseCase:        orgUseCase,
		orgMetricsUseCase: orgMetricsUseCase,
		logger:            log.NewHelper(log.With(logger, "component", "biz/prometheus")),
	}

	registryManager := loadPrometheusRegistries(conf, orgMetricsUseCase, logger)
	useCase.registryManager = registryManager

	return useCase
}

// loadPrometheusRegistries loads the prometheus registries from the configuration
func loadPrometheusRegistries(conf []*conf.PrometheusIntegrationSpec, useCase *OrgMetricsUseCase, logger log.Logger) *chainloopprometheus.ChainloopRegistryManager {
	manager := chainloopprometheus.NewChainloopRegistryManager()

	for _, spec := range conf {
		reg := registry.NewPrometheusRegistry(spec.GetOrgName(), useCase, logger)
		manager.AddRegistry(reg)
	}

	return manager
}

// Record an attestation if the run exists and there is a registry for the organization
func (uc *PrometheusUseCase) ObserveAttestationIfNeeded(ctx context.Context, run *WorkflowRun, status WorkflowRunStatus) bool {
	if run == nil || run.Workflow == nil {
		return false
	}

	workflow := run.Workflow
	orgID := workflow.OrgID

	org, err := uc.orgUseCase.FindByID(ctx, orgID.String())
	if err != nil {
		return false
	}

	if !uc.OrganizationHasRegistry(org.Name) {
		return false
	}

	err = uc.observeAttestation(org.Name, workflow.Name, status, run.RunnerType, run.CreatedAt)
	return err == nil
}

// OrganizationHasRegistry checks if an organization has a registry
func (uc *PrometheusUseCase) observeAttestation(orgName, wfName string, status WorkflowRunStatus, runnerType string, startTime *time.Time) error {
	if orgName == "" || wfName == "" || status == "" || startTime == nil {
		return NewErrValidationStr("orgName, wfName, and state must be non-empty")
	}

	reg := uc.GetRegistryByOrganizationName(orgName)
	if reg == nil {
		return NewErrNotFound("registry not found for organization")
	}

	duration := time.Since(*startTime).Seconds()
	reg.WorkflowRunDurationSeconds.With(prometheus.Labels{"org": orgName, "workflow": wfName, "status": string(status), "runner": runnerType}).Observe(duration)
	return nil
}

// OrganizationHasRegistry checks if an organization has a registry
func (uc *PrometheusUseCase) OrganizationHasRegistry(orgName string) bool {
	return uc.registryManager.GetRegistryByName(orgName) != nil
}

// GetRegistryByOrganizationName returns a registry by organization name
func (uc *PrometheusUseCase) GetRegistryByOrganizationName(orgName string) *registry.PrometheusRegistry {
	return uc.registryManager.GetRegistryByName(orgName)
}
