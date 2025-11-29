//
// Copyright 2024-2025 The Chainloop Authors.
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

package runners

import (
	"fmt"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

type TektonPipeline struct{}

func NewTektonPipeline() *TektonPipeline {
	return &TektonPipeline{}
}

func (r *TektonPipeline) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_TEKTON_PIPELINE
}

// CheckEnv detects if we're running in a Tekton environment
// by checking for the existence of Tekton-specific directories
func (r *TektonPipeline) CheckEnv() bool {
	// Check for /tekton/results directory (most reliable indicator)
	if _, err := os.Stat("/tekton/results"); err == nil {
		return true
	}
	return false
}

func (r *TektonPipeline) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{
		// PipelineRun context (optional - only present when running in a Pipeline)
		{"TEKTON_PIPELINE_RUN", true},
		{"TEKTON_PIPELINE_RUN_UID", true},
		{"TEKTON_PIPELINE", true},

		// TaskRun context (optional - should be set by users following best practices)
		{"TEKTON_TASKRUN_NAME", true},
		{"TEKTON_TASKRUN_UID", true},
		{"TEKTON_TASK_NAME", true},

		// Namespace (optional - can be read from service account or set explicitly)
		{"TEKTON_NAMESPACE", true},
	}
}

func (r *TektonPipeline) RunURI() string {
	// Priority 1: If we have PipelineRun context, construct PipelineRun URL
	if pipelineRun := os.Getenv("TEKTON_PIPELINE_RUN"); pipelineRun != "" {
		namespace := r.getNamespace()
		if namespace != "" {
			// Use Tekton Dashboard URL format
			// Users can customize dashboard URL via environment variable
			dashboardURL := os.Getenv("TEKTON_DASHBOARD_URL")
			if dashboardURL == "" {
				dashboardURL = "https://dashboard.tekton.dev"
			}
			return fmt.Sprintf("%s/#/namespaces/%s/pipelineruns/%s", dashboardURL, namespace, pipelineRun)
		}
	}

	// Priority 2: If we have TaskRun context, construct TaskRun URL
	if taskRun := os.Getenv("TEKTON_TASKRUN_NAME"); taskRun != "" {
		namespace := r.getNamespace()
		if namespace != "" {
			dashboardURL := os.Getenv("TEKTON_DASHBOARD_URL")
			if dashboardURL == "" {
				dashboardURL = "https://dashboard.tekton.dev"
			}
			return fmt.Sprintf("%s/#/namespaces/%s/taskruns/%s", dashboardURL, namespace, taskRun)
		}
	}

	return ""
}

// getNamespace attempts to get the namespace from multiple sources
func (r *TektonPipeline) getNamespace() string {
	// Priority 1: Environment variable
	if namespace := os.Getenv("TEKTON_NAMESPACE"); namespace != "" {
		return namespace
	}

	// Priority 2: Read from service account (standard Kubernetes location)
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return string(data)
	}

	return ""
}

func (r *TektonPipeline) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}

func (r *TektonPipeline) WorkflowFilePath() string {
	// Tekton doesn't have a single workflow file path concept
	// Tasks and Pipelines are defined as Kubernetes resources
	return ""
}

func (r *TektonPipeline) IsAuthenticated() bool {
	// No OIDC support initially
	return false
}

func (r *TektonPipeline) Environment() RunnerEnvironment {
	// Could be enhanced to detect managed Tekton services (e.g., OpenShift Pipelines)
	// For now, return Unknown
	return Unknown
}
