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
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type tektonPipelineTestSuite struct {
	suite.Suite
	runner *TektonPipeline
}

func (s *tektonPipelineTestSuite) SetupTest() {
	s.runner = NewTektonPipeline()
}

func (s *tektonPipelineTestSuite) TestID() {
	assert.Equal(s.T(), schemaapi.CraftingSchema_Runner_TEKTON_PIPELINE, s.runner.ID())
	assert.Equal(s.T(), "TEKTON_PIPELINE", s.runner.ID().String())
}

func (s *tektonPipelineTestSuite) TestCheckEnv() {
	// CheckEnv should return false in normal test environment (no /tekton directory)
	assert.False(s.T(), s.runner.CheckEnv())

	// Note: Testing true case would require mocking filesystem or integration tests
	// The /tekton/results directory only exists in actual Tekton task/pipeline executions
}

func (s *tektonPipelineTestSuite) TestListEnvVars() {
	envVars := s.runner.ListEnvVars()
	assert.Greater(s.T(), len(envVars), 0)

	// All environment variables should be optional (Tekton doesn't auto-inject them)
	for _, envVar := range envVars {
		assert.True(s.T(), envVar.Optional, "Expected %s to be optional", envVar.Name)
	}

	// Check for expected environment variable names
	expectedVars := []string{
		"TEKTON_PIPELINE_RUN",
		"TEKTON_PIPELINE_RUN_UID",
		"TEKTON_PIPELINE",
		"TEKTON_TASKRUN_NAME",
		"TEKTON_TASKRUN_UID",
		"TEKTON_TASK_NAME",
		"TEKTON_NAMESPACE",
	}

	envVarMap := make(map[string]bool)
	for _, envVar := range envVars {
		envVarMap[envVar.Name] = true
	}

	for _, expected := range expectedVars {
		assert.True(s.T(), envVarMap[expected], "Expected %s to be in list", expected)
	}
}

func (s *tektonPipelineTestSuite) TestRunURI_PipelineRun() {
	// Set up PipelineRun environment
	s.T().Setenv("TEKTON_PIPELINE_RUN", "my-pipeline-run-123")
	s.T().Setenv("TEKTON_NAMESPACE", "production")

	uri := s.runner.RunURI()
	expected := "https://dashboard.tekton.dev/#/namespaces/production/pipelineruns/my-pipeline-run-123"
	assert.Equal(s.T(), expected, uri)
}

func (s *tektonPipelineTestSuite) TestRunURI_TaskRun() {
	// Set up TaskRun environment (no PipelineRun)
	s.T().Setenv("TEKTON_TASKRUN_NAME", "my-taskrun-456")
	s.T().Setenv("TEKTON_NAMESPACE", "default")

	uri := s.runner.RunURI()
	expected := "https://dashboard.tekton.dev/#/namespaces/default/taskruns/my-taskrun-456"
	assert.Equal(s.T(), expected, uri)
}

func (s *tektonPipelineTestSuite) TestRunURI_CustomDashboard() {
	// Test custom dashboard URL
	s.T().Setenv("TEKTON_PIPELINE_RUN", "my-run")
	s.T().Setenv("TEKTON_NAMESPACE", "default")
	s.T().Setenv("TEKTON_DASHBOARD_URL", "https://tekton.example.com")

	uri := s.runner.RunURI()
	expected := "https://tekton.example.com/#/namespaces/default/pipelineruns/my-run"
	assert.Equal(s.T(), expected, uri)
}

func (s *tektonPipelineTestSuite) TestRunURI_PipelineRunPriority() {
	// When both PipelineRun and TaskRun are present, PipelineRun should take priority
	s.T().Setenv("TEKTON_PIPELINE_RUN", "pipeline-run-123")
	s.T().Setenv("TEKTON_TASKRUN_NAME", "taskrun-456")
	s.T().Setenv("TEKTON_NAMESPACE", "default")

	uri := s.runner.RunURI()
	// Should contain pipelineruns, not taskruns
	assert.Contains(s.T(), uri, "pipelineruns/pipeline-run-123")
	assert.NotContains(s.T(), uri, "taskruns")
}

func (s *tektonPipelineTestSuite) TestRunURI_NoContext() {
	// Test with no context variables set
	uri := s.runner.RunURI()
	assert.Equal(s.T(), "", uri)
}

func (s *tektonPipelineTestSuite) TestRunURI_MissingNamespace() {
	// Test with run name but no namespace
	s.T().Setenv("TEKTON_PIPELINE_RUN", "my-run")
	// No TEKTON_NAMESPACE set and no service account file

	uri := s.runner.RunURI()
	// Should return empty string when namespace cannot be determined
	assert.Equal(s.T(), "", uri)
}

func (s *tektonPipelineTestSuite) TestWorkflowFilePath() {
	// Tekton doesn't have workflow file paths
	assert.Equal(s.T(), "", s.runner.WorkflowFilePath())
}

func (s *tektonPipelineTestSuite) TestIsAuthenticated() {
	// No OIDC support initially
	assert.False(s.T(), s.runner.IsAuthenticated())
}

func (s *tektonPipelineTestSuite) TestEnvironment() {
	// Should return Unknown
	assert.Equal(s.T(), Unknown, s.runner.Environment())
}

func (s *tektonPipelineTestSuite) TestResolveEnvVars_Partial() {
	// Set some environment variables
	s.T().Setenv("TEKTON_PIPELINE_RUN", "my-run")
	s.T().Setenv("TEKTON_NAMESPACE", "default")

	resolved, errors := s.runner.ResolveEnvVars()

	// Should succeed since all vars are optional
	assert.Nil(s.T(), errors)
	assert.Equal(s.T(), "my-run", resolved["TEKTON_PIPELINE_RUN"])
	assert.Equal(s.T(), "default", resolved["TEKTON_NAMESPACE"])
}

func (s *tektonPipelineTestSuite) TestResolveEnvVars_Full() {
	// Set all environment variables
	testEnvVars := map[string]string{
		"TEKTON_PIPELINE_RUN":     "pipeline-run-123",
		"TEKTON_PIPELINE_RUN_UID": "abc-123-def",
		"TEKTON_PIPELINE":         "my-pipeline",
		"TEKTON_TASKRUN_NAME":     "taskrun-456",
		"TEKTON_TASKRUN_UID":      "xyz-789-uvw",
		"TEKTON_TASK_NAME":        "my-task",
		"TEKTON_NAMESPACE":        "production",
	}

	for k, v := range testEnvVars {
		s.T().Setenv(k, v)
	}

	resolved, errors := s.runner.ResolveEnvVars()

	// Should succeed with all variables resolved
	assert.Nil(s.T(), errors)
	for k, v := range testEnvVars {
		assert.Equal(s.T(), v, resolved[k], "Expected %s to be %s", k, v)
	}
}

func (s *tektonPipelineTestSuite) TestResolveEnvVars_Empty() {
	// Test with no environment variables set
	resolved, errors := s.runner.ResolveEnvVars()

	// Should succeed since all vars are optional
	assert.Nil(s.T(), errors)
	// Resolved map should be empty (no env vars set)
	assert.Empty(s.T(), resolved)
}

func (s *tektonPipelineTestSuite) TestGetNamespace_FromEnvVar() {
	// Test namespace resolution from environment variable
	s.T().Setenv("TEKTON_NAMESPACE", "my-namespace")

	namespace := s.runner.getNamespace()
	assert.Equal(s.T(), "my-namespace", namespace)
}

func (s *tektonPipelineTestSuite) TestGetNamespace_NoSource() {
	// Test with no namespace sources available
	// (service account file doesn't exist in test environment)
	namespace := s.runner.getNamespace()
	assert.Equal(s.T(), "", namespace)
}

func TestTektonPipelineRunner(t *testing.T) {
	suite.Run(t, new(tektonPipelineTestSuite))
}
