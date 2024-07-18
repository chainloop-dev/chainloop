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

package prometheuscollector

import (
	"slices"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/prometheus/client_golang/prometheus"
)

// ChainloopCollector is an interface for a collector that collects metrics for Chainloop
// It extends the prometheus.Collector interface plus two additional methods
type ChainloopCollector interface {
	prometheus.Collector
}

// BaseChainloopCollector is a base implementation of the ChainloopCollector interface
type BaseChainloopCollector struct {
	orgName  string
	gatherer ChainloopMetricsGatherer
	// Metrics
	workflowLastRunSuccessful *prometheus.GaugeVec
	// Others
	logger *log.Helper
}

// NewBaseChainloopCollector creates a new BaseChainloopCollector with basic metrics
func NewBaseChainloopCollector(orgName string, gatherer ChainloopMetricsGatherer, logger log.Logger) *BaseChainloopCollector {
	return &BaseChainloopCollector{
		orgName:  orgName,
		gatherer: gatherer,
		workflowLastRunSuccessful: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "chainloop_workflow_up",
			Help: "The last state of the workflows by their last run",
		}, []string{"org_name", "workflow_name"}),
		logger: log.NewHelper(log.With(logger, "component", "collector/prometheus")),
	}
}

func (bcc *BaseChainloopCollector) Describe(ch chan<- *prometheus.Desc) {
	bcc.workflowLastRunSuccessful.Describe(ch)
}

func (bcc *BaseChainloopCollector) Collect(ch chan<- prometheus.Metric) {
	wfReports, err := bcc.gatherer.GetLastWorkflowStatusByRun(bcc.orgName)
	if err != nil {
		bcc.logger.Warnf("error getting last workflow status by run for organization [%v]: %v", bcc.orgName, err)
		return
	}

	for _, r := range wfReports {
		if slices.Contains(notSuccessfulStatus, r.Status) {
			bcc.workflowLastRunSuccessful.WithLabelValues(r.OrgName, r.WorkflowName).Set(0)
		} else {
			bcc.workflowLastRunSuccessful.WithLabelValues(r.OrgName, r.WorkflowName).Set(1)
		}
	}

	bcc.workflowLastRunSuccessful.Collect(ch)
}
