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

// ChainloopMetricsGatherer is an interface for a gatherer that gathers metrics for Chainloop prometheus collector
type ChainloopMetricsGatherer interface {
	GetLastWorkflowStatusByRun(orgName string) ([]*WorkflowLastStatusByRunReport, error)
}

// notSuccessfulStatus is a list of statuses that are not considered successful
var notSuccessfulStatus = []string{
	"error",
	"canceled",
	"expired",
}

// WorkflowLastStatusByRunReport is a report of the status of a workflow by its last run
type WorkflowLastStatusByRunReport struct {
	WorkflowName string `json:"workflow_name"`
	OrgName      string `json:"org_name"`
	Status       string `json:"status"`
}
