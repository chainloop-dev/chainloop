//
// Copyright 2025 The Chainloop Authors.
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
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

const (
	// Default path for Downward API labels
	defaultLabelsPath = "/etc/podinfo/labels"
	// Default Tekton dashboard URL
	defaultDashboardURL = "https://dashboard.tekton.dev"
)

type TektonPipeline struct {
	// Path to the Downward API labels file (configurable for testing)
	labelsPath string
}

func NewTektonPipeline() *TektonPipeline {
	return &TektonPipeline{
		labelsPath: defaultLabelsPath,
	}
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

// ListEnvVars returns environment variables collected from Downward API labels
func (r *TektonPipeline) ListEnvVars() []*EnvVarDefinition {
	// Parse labels and convert to environment variable definitions
	labels := r.parseLabels()
	if len(labels) == 0 {
		return []*EnvVarDefinition{}
	}

	var envVars []*EnvVarDefinition

	// Map Tekton labels to environment variables (all optional)
	labelMappings := map[string]string{
		"tekton.dev/pipelineRun":    "TEKTON_PIPELINE_RUN",
		"tekton.dev/pipelineRunUID": "TEKTON_PIPELINE_RUN_UID",
		"tekton.dev/pipeline":       "TEKTON_PIPELINE",
		"tekton.dev/taskRun":        "TEKTON_TASKRUN_NAME",
		"tekton.dev/taskRunUID":     "TEKTON_TASKRUN_UID",
		"tekton.dev/task":           "TEKTON_TASK_NAME",
	}

	for labelKey, envVarName := range labelMappings {
		if _, exists := labels[labelKey]; exists {
			envVars = append(envVars, &EnvVarDefinition{
				Name:     envVarName,
				Optional: true,
			})
		}
	}

	// Add namespace if available
	if ns := r.getNamespace(); ns != "" {
		envVars = append(envVars, &EnvVarDefinition{
			Name:     "TEKTON_NAMESPACE",
			Optional: true,
		})
	}

	return envVars
}

// parseLabels reads and parses the Downward API labels file
// Returns a map of label key-value pairs
func (r *TektonPipeline) parseLabels() map[string]string {
	labels := make(map[string]string)

	data, err := os.ReadFile(r.labelsPath)
	if err != nil {
		return labels
	}

	// Parse labels in format: key="value"
	// Labels are separated by newlines
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split on first = to get key and quoted value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"`)
		labels[key] = value
	}

	return labels
}

func (r *TektonPipeline) RunURI() string {
	labels := r.parseLabels()
	namespace := r.getNamespace()

	if namespace == "" {
		return ""
	}

	// Get dashboard URL from environment variable or use default
	dashboardURL := os.Getenv("TEKTON_DASHBOARD_URL")
	if dashboardURL == "" {
		dashboardURL = defaultDashboardURL
	}

	// Priority 1: If we have PipelineRun context, construct PipelineRun URL
	if pipelineRun, ok := labels["tekton.dev/pipelineRun"]; ok && pipelineRun != "" {
		return fmt.Sprintf("%s/#/namespaces/%s/pipelineruns/%s", dashboardURL, namespace, pipelineRun)
	}

	// Priority 2: If we have TaskRun context, construct TaskRun URL
	if taskRun, ok := labels["tekton.dev/taskRun"]; ok && taskRun != "" {
		return fmt.Sprintf("%s/#/namespaces/%s/taskruns/%s", dashboardURL, namespace, taskRun)
	}

	return ""
}

// getNamespace attempts to get the namespace from the service account
func (r *TektonPipeline) getNamespace() string {
	// Read from service account (standard Kubernetes location)
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return strings.TrimSpace(string(data))
	}

	return ""
}

func (r *TektonPipeline) ResolveEnvVars() (map[string]string, []*error) {
	result := make(map[string]string)
	labels := r.parseLabels()

	// Map Tekton labels to environment variable names
	labelMappings := map[string]string{
		"tekton.dev/pipelineRun":    "TEKTON_PIPELINE_RUN",
		"tekton.dev/pipelineRunUID": "TEKTON_PIPELINE_RUN_UID",
		"tekton.dev/pipeline":       "TEKTON_PIPELINE",
		"tekton.dev/taskRun":        "TEKTON_TASKRUN_NAME",
		"tekton.dev/taskRunUID":     "TEKTON_TASKRUN_UID",
		"tekton.dev/task":           "TEKTON_TASK_NAME",
	}

	for labelKey, envVarName := range labelMappings {
		if value, ok := labels[labelKey]; ok && value != "" {
			result[envVarName] = value
		}
	}

	// Add namespace if available
	if ns := r.getNamespace(); ns != "" {
		result["TEKTON_NAMESPACE"] = ns
	}

	// No errors since all variables are optional
	return result, nil
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
