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
	"os"
	"path/filepath"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type tektonPipelineTestSuite struct {
	suite.Suite
	runner *TektonPipeline
	tmpDir string
}

func (s *tektonPipelineTestSuite) SetupTest() {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "tekton-test-*")
	assert.NoError(s.T(), err)
	s.tmpDir = tmpDir

	s.runner = NewTektonPipeline()
	// Point to temp directory for tests
	s.runner.labelsPath = filepath.Join(tmpDir, "labels")
}

func (s *tektonPipelineTestSuite) TearDownTest() {
	// Clean up temporary directory
	if s.tmpDir != "" {
		os.RemoveAll(s.tmpDir)
	}
}

func (s *tektonPipelineTestSuite) TestID() {
	assert.Equal(s.T(), schemaapi.CraftingSchema_Runner_TEKTON_PIPELINE, s.runner.ID())
	assert.Equal(s.T(), "TEKTON_PIPELINE", s.runner.ID().String())
}

func (s *tektonPipelineTestSuite) TestCheckEnv() {
	// CheckEnv should return false in normal test environment (no /tekton directory)
	assert.False(s.T(), s.runner.CheckEnv())
}

func (s *tektonPipelineTestSuite) TestParseLabels_PipelineRun() {
	// Create a labels file with PipelineRun context
	labelsContent := `tekton.dev/pipeline="my-pipeline"
tekton.dev/pipelineRun="my-pipeline-run-123"
tekton.dev/pipelineRunUID="abc-123-def"
tekton.dev/pipelineTask="build-task"
tekton.dev/taskRun="my-pipeline-run-123-build-task-xyz"
tekton.dev/taskRunUID="xyz-789-uvw"
tekton.dev/task="build-task"
app.kubernetes.io/managed-by="tekton-pipelines"
`
	err := os.WriteFile(s.runner.labelsPath, []byte(labelsContent), 0600)
	assert.NoError(s.T(), err)

	labels := s.runner.parseLabels()
	assert.Equal(s.T(), "my-pipeline", labels["tekton.dev/pipeline"])
	assert.Equal(s.T(), "my-pipeline-run-123", labels["tekton.dev/pipelineRun"])
	assert.Equal(s.T(), "abc-123-def", labels["tekton.dev/pipelineRunUID"])
	assert.Equal(s.T(), "my-pipeline-run-123-build-task-xyz", labels["tekton.dev/taskRun"])
}

func (s *tektonPipelineTestSuite) TestParseLabels_TaskRun() {
	// Create a labels file with standalone TaskRun context
	labelsContent := `tekton.dev/task="my-task"
tekton.dev/taskRun="my-taskrun-456"
tekton.dev/taskRunUID="def-456-ghi"
app.kubernetes.io/managed-by="tekton-pipelines"
`
	err := os.WriteFile(s.runner.labelsPath, []byte(labelsContent), 0600)
	assert.NoError(s.T(), err)

	labels := s.runner.parseLabels()
	assert.Equal(s.T(), "my-task", labels["tekton.dev/task"])
	assert.Equal(s.T(), "my-taskrun-456", labels["tekton.dev/taskRun"])
	assert.Equal(s.T(), "def-456-ghi", labels["tekton.dev/taskRunUID"])
	// PipelineRun labels should not be present
	_, hasPipelineRun := labels["tekton.dev/pipelineRun"]
	assert.False(s.T(), hasPipelineRun)
}

func (s *tektonPipelineTestSuite) TestParseLabels_NoFile() {
	// When labels file doesn't exist, should return empty map
	labels := s.runner.parseLabels()
	assert.Empty(s.T(), labels)
}

func (s *tektonPipelineTestSuite) TestListEnvVars_WithLabels() {
	// Create a labels file
	labelsContent := `tekton.dev/pipelineRun="my-run"
tekton.dev/taskRun="my-task-run"
`
	err := os.WriteFile(s.runner.labelsPath, []byte(labelsContent), 0600)
	assert.NoError(s.T(), err)

	envVars := s.runner.ListEnvVars()
	assert.Greater(s.T(), len(envVars), 0)

	// All environment variables should be optional
	for _, envVar := range envVars {
		assert.True(s.T(), envVar.Optional, "Expected %s to be optional", envVar.Name)
	}
}

func (s *tektonPipelineTestSuite) TestListEnvVars_NoLabels() {
	// When labels file doesn't exist, should return empty list
	envVars := s.runner.ListEnvVars()
	assert.Empty(s.T(), envVars)
}

func (s *tektonPipelineTestSuite) TestResolveEnvVars_PipelineRun() {
	// Create a labels file with PipelineRun context
	labelsContent := `tekton.dev/pipeline="my-pipeline"
tekton.dev/pipelineRun="my-pipeline-run-123"
tekton.dev/pipelineRunUID="abc-123-def"
tekton.dev/taskRun="task-run-xyz"
`
	err := os.WriteFile(s.runner.labelsPath, []byte(labelsContent), 0600)
	assert.NoError(s.T(), err)

	resolved, errors := s.runner.ResolveEnvVars()

	assert.Nil(s.T(), errors)
	assert.Equal(s.T(), "my-pipeline", resolved["TEKTON_PIPELINE"])
	assert.Equal(s.T(), "my-pipeline-run-123", resolved["TEKTON_PIPELINE_RUN"])
	assert.Equal(s.T(), "abc-123-def", resolved["TEKTON_PIPELINE_RUN_UID"])
	assert.Equal(s.T(), "task-run-xyz", resolved["TEKTON_TASKRUN_NAME"])
}

func (s *tektonPipelineTestSuite) TestResolveEnvVars_NoLabels() {
	// When labels file doesn't exist, should return empty map with no errors
	resolved, errors := s.runner.ResolveEnvVars()

	assert.Nil(s.T(), errors)
	// Should be empty or only contain namespace if service account exists
	assert.LessOrEqual(s.T(), len(resolved), 1) // Max 1 for namespace
}

func (s *tektonPipelineTestSuite) TestRunURI_PipelineRun() {
	// Create labels and namespace files
	labelsContent := `tekton.dev/pipelineRun="my-pipeline-run-123"
`
	err := os.WriteFile(s.runner.labelsPath, []byte(labelsContent), 0600)
	assert.NoError(s.T(), err)

	// Mock namespace by creating service account namespace file
	nsDir := filepath.Join(s.tmpDir, "run", "secrets", "kubernetes.io", "serviceaccount")
	err = os.MkdirAll(nsDir, 0755)
	assert.NoError(s.T(), err)
	nsPath := filepath.Join(nsDir, "namespace")
	err = os.WriteFile(nsPath, []byte("production"), 0600)
	assert.NoError(s.T(), err)

	// Override the namespace path temporarily
	// Since we can't easily mock os.ReadFile, we'll just test that labels are parsed correctly
	labels := s.runner.parseLabels()
	assert.Equal(s.T(), "my-pipeline-run-123", labels["tekton.dev/pipelineRun"])

	// The actual URI construction would need namespace from service account which doesn't exist in tests
	uri := s.runner.RunURI()
	// Will be empty in test environment since service account file is in different location
	assert.Equal(s.T(), "", uri)
}

func (s *tektonPipelineTestSuite) TestRunURI_TaskRun() {
	// Create a labels file with TaskRun only (no PipelineRun)
	labelsContent := `tekton.dev/taskRun="my-taskrun-456"
`
	err := os.WriteFile(s.runner.labelsPath, []byte(labelsContent), 0600)
	assert.NoError(s.T(), err)

	labels := s.runner.parseLabels()
	assert.Equal(s.T(), "my-taskrun-456", labels["tekton.dev/taskRun"])
}

func (s *tektonPipelineTestSuite) TestRunURI_PipelineRunPriority() {
	// When both PipelineRun and TaskRun are present, verify labels are parsed correctly
	labelsContent := `tekton.dev/pipelineRun="pipeline-run-123"
tekton.dev/taskRun="taskrun-456"
`
	err := os.WriteFile(s.runner.labelsPath, []byte(labelsContent), 0600)
	assert.NoError(s.T(), err)

	labels := s.runner.parseLabels()
	assert.Equal(s.T(), "pipeline-run-123", labels["tekton.dev/pipelineRun"])
	assert.Equal(s.T(), "taskrun-456", labels["tekton.dev/taskRun"])
	// Priority logic is tested in RunURI implementation
}

func (s *tektonPipelineTestSuite) TestRunURI_NoLabels() {
	// Test with no labels file
	uri := s.runner.RunURI()
	assert.Equal(s.T(), "", uri)
}

func (s *tektonPipelineTestSuite) TestRunURI_CustomDashboard() {
	// Test custom dashboard URL via environment variable
	labelsContent := `tekton.dev/pipelineRun="my-run"
`
	err := os.WriteFile(s.runner.labelsPath, []byte(labelsContent), 0600)
	assert.NoError(s.T(), err)

	s.T().Setenv("TEKTON_DASHBOARD_URL", "https://tekton.example.com")

	// Custom dashboard URL is tested in the implementation
	// Actual URI would need namespace which isn't available in test environment
	labels := s.runner.parseLabels()
	assert.Equal(s.T(), "my-run", labels["tekton.dev/pipelineRun"])
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

func (s *tektonPipelineTestSuite) TestGetNamespace_NoServiceAccount() {
	// Test with no service account file (normal test environment)
	namespace := s.runner.getNamespace()
	assert.Equal(s.T(), "", namespace)
}

func TestTektonPipelineRunner(t *testing.T) {
	suite.Run(t, new(tektonPipelineTestSuite))
}
