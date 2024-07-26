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
	"github.com/go-kratos/kratos/v2/log"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/metrics/prometheus/collector"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusRegistry is a wrapper around a prometheus registry that also holds a list of ChainloopCollectors
type PrometheusRegistry struct {
	*prometheus.Registry
	Name               string
	chainloopCollector *collector.ChainloopCollector

	// metrics
	WorkflowRunDurationSeconds *prometheus.HistogramVec
}

var workflowRunDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name: "chainloop_wf_run_duration_seconds",
	Help: "Duration of a workflow runs in seconds.",
	// TODO: tweak the buckets
	Buckets: prometheus.DefBuckets,
}, []string{"org", "workflow", "status"})

// NewPrometheusRegistry creates a new Prometheus registry with a given ID and collector
func NewPrometheusRegistry(name string, gatherer collector.ChainloopMetricsGatherer, logger log.Logger) *PrometheusRegistry {
	reg := prometheus.NewRegistry()

	// Collector of metrics stored in DB
	bcc := collector.NewChainloopCollector(name, gatherer, logger)

	reg.MustRegister(bcc)

	// Custom metrics that come from the business logic
	reg.MustRegister(workflowRunDurationSeconds)

	return &PrometheusRegistry{
		Name:                       name,
		Registry:                   reg,
		chainloopCollector:         bcc,
		WorkflowRunDurationSeconds: workflowRunDurationSeconds,
	}
}
