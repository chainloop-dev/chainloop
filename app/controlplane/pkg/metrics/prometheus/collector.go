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
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/prometheus/client_golang/prometheus"
)

// ChainloopMetricsGatherer is an interface for a gatherer that gathers metrics for Chainloop prometheus collector
type ChainloopMetricsGatherer interface {
	GetLastWorkflowStatusByRun(ctx context.Context, orgName string) ([]*WorkflowLastStatusByRunReport, error)
}

// WorkflowLastStatusByRunReport is a report of the status of a workflow by its last run
type WorkflowLastStatusByRunReport struct {
	WorkflowName string `json:"workflow_name"`
	OrgName      string `json:"org_name"`
	Status       string `json:"status"`
}

// ChainloopCollector is a base implementation of the ChainloopCollector interface
type ChainloopCollector struct {
	orgName  string
	gatherer ChainloopMetricsGatherer
	// Metrics
	workflowLastRunSuccessful *prometheus.GaugeVec
	// Others
	logger *log.Helper
}

// NewChainloopCollector creates a new ChainloopCollector with basic metrics
func NewChainloopCollector(orgName string, gatherer ChainloopMetricsGatherer, logger log.Logger) *ChainloopCollector {
	return &ChainloopCollector{
		orgName:  orgName,
		gatherer: gatherer,
		workflowLastRunSuccessful: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "chainloop_workflow_up",
			Help: "Indicate if the last run was successful.",
		}, []string{"org", "workflow"}),
		logger: log.NewHelper(log.With(logger, "component", "collector/prometheus")),
	}
}

func (bcc *ChainloopCollector) Describe(ch chan<- *prometheus.Desc) {
	bcc.workflowLastRunSuccessful.Describe(ch)
}

func (bcc *ChainloopCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	wfReports, err := bcc.gatherer.GetLastWorkflowStatusByRun(ctx, bcc.orgName)
	if err != nil {
		bcc.logger.Warnf("error getting last workflow status by run for organization [%v]: %v", bcc.orgName, err)
		return
	}

	for _, r := range wfReports {
		if r.Status == "success" {
			bcc.workflowLastRunSuccessful.WithLabelValues(r.OrgName, r.WorkflowName).Set(1)
		} else {
			bcc.workflowLastRunSuccessful.WithLabelValues(r.OrgName, r.WorkflowName).Set(0)
		}
	}

	bcc.workflowLastRunSuccessful.Collect(ch)
}
