//
// Copyright 2023 The Chainloop Authors.
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

package crafter_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/runners"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/statemanager/filesystem"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	mUploader "github.com/chainloop-dev/chainloop/internal/casclient/mocks"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

type crafterSuite struct {
	suite.Suite
	// initOpts
	workflowMetadata *v1.WorkflowMetadata
	repoPath         string
	repoHead         string
}

func (s *crafterSuite) TestInit() {
	testCases := []struct {
		name             string
		contractPath     string
		workingDir       string
		workflowMetadata *v1.WorkflowMetadata
		wantErr          bool
		wantRepoDigest   bool
		dryRun           bool
	}{
		{
			name:             "happy path inside a git repo",
			contractPath:     "testdata/contracts/empty_generic.yaml",
			workflowMetadata: s.workflowMetadata,
			dryRun:           true,
			workingDir:       s.repoPath,
			wantRepoDigest:   true,
		},
		{
			name:             "happy path outside a git repo",
			contractPath:     "testdata/contracts/empty_generic.yaml",
			workflowMetadata: s.workflowMetadata,
			workingDir:       s.T().TempDir(),
			dryRun:           true,
		},
		{
			name:             "missing metadata",
			contractPath:     "testdata/contracts/empty_generic.yaml",
			workflowMetadata: nil,
			wantErr:          true,
		},
		{
			name:             "required github action env (dry run)",
			contractPath:     "testdata/contracts/empty_github.yaml",
			workflowMetadata: s.workflowMetadata,
			wantErr:          false,
			dryRun:           true,
			workingDir:       s.T().TempDir(),
		},
		{
			name:             "with annotations",
			contractPath:     "testdata/contracts/with_material_annotations.yaml",
			workflowMetadata: s.workflowMetadata,
			workingDir:       s.T().TempDir(),
			dryRun:           true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			contract, err := crafter.LoadSchema(tc.contractPath)
			require.NoError(s.T(), err)

			runner := crafter.NewRunner(contract.GetRunner().GetType())
			// Make sure that the tests context indicate that we are not in a CI
			// this makes the github action runner context to fail
			c, err := newInitializedCrafter(s.T(), tc.contractPath, tc.workflowMetadata, tc.dryRun, tc.workingDir, runner)
			if tc.wantErr {
				s.Error(err)
				return
			}

			s.NoError(err)

			want := &v1.CraftingState{
				InputSchema: contract,
				Attestation: &v1.Attestation{
					Workflow:   tc.workflowMetadata,
					RunnerType: contract.GetRunner().GetType(),
				},
				DryRun: tc.dryRun,
			}

			if tc.wantRepoDigest {
				want.Attestation.Head = &v1.Commit{Hash: s.repoHead, AuthorEmail: "john@doe.org", AuthorName: "John Doe", Message: "test commit"}
				c.CraftingState.Attestation.Head.Date = nil
			}

			// reset to nil to easily compare them
			s.NotNil(c.CraftingState.Attestation.InitializedAt)
			c.CraftingState.Attestation.InitializedAt = nil
			c.CraftingState.Attestation.RunnerUrl = ""

			// Check state
			if ok := proto.Equal(want, c.CraftingState); !ok {
				s.Fail(fmt.Sprintf("These two protobuf messages are not equal:\nexpected: %v\nactual:  %v", want, c.CraftingState))
			}
		})
	}
}

type testingCrafter struct {
	*crafter.Crafter
}

func testingStateManager(t *testing.T, statePath string) crafter.StateManager {
	stateManager, err := filesystem.New(statePath)
	require.NoError(t, err)
	return stateManager
}

func newInitializedCrafter(t *testing.T, contractPath string, wfMeta *v1.WorkflowMetadata,
	dryRun bool,
	workingDir string,
	runner crafter.SupportedRunner,
) (*testingCrafter, error) {
	opts := []crafter.NewOpt{}
	if workingDir != "" {
		opts = append(opts, crafter.WithWorkingDirPath(workingDir))
	}

	if runner == nil {
		runner = runners.NewGeneric()
	}

	statePath := fmt.Sprintf("%s/attestation.json", t.TempDir())
	c, err := crafter.NewCrafter(testingStateManager(t, statePath), opts...)
	require.NoError(t, err)
	contract, err := crafter.LoadSchema(contractPath)
	if err != nil {
		return nil, err
	}

	if err = c.Init(context.Background(), &crafter.InitOpts{
		SchemaV1: contract, WfInfo: wfMeta, DryRun: dryRun,
		AttestationID: "",
		Runner:        runner}); err != nil {
		return nil, err
	}

	return &testingCrafter{c}, nil
}

func (s *crafterSuite) TestLoadSchema() {
	testCases := []struct {
		name         string
		contractPath string
		wantErr      bool
	}{
		{
			name:         "yaml",
			contractPath: "testdata/contracts/empty_github.yaml",
		},
		{
			name:         "json",
			contractPath: "testdata/contracts/empty_github.json",
		},
		{
			name:         "cue",
			contractPath: "testdata/contracts/empty_github.cue",
		},
		{
			name:         "unsupported",
			contractPath: "testdata/contracts/invalid.xml",
			wantErr:      true,
		},
		{
			name:         "non existing",
			contractPath: "testdata/contracts/invalid.yaml",
			wantErr:      true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			got, err := crafter.LoadSchema(tc.contractPath)
			if tc.wantErr {
				s.Error(err)
				return
			}

			want := &schemaapi.CraftingSchema{
				SchemaVersion: "v1",
				Runner: &schemaapi.CraftingSchema_Runner{
					Type: schemaapi.CraftingSchema_Runner_GITHUB_ACTION,
				},
			}

			// Check state
			if ok := proto.Equal(want, got); !ok {
				s.Fail(fmt.Sprintf("These two protobuf messages are not equal:\nexpected: %v\nactual:  %v", want, got))
			}
		})
	}
}

func (s *crafterSuite) TestResolveEnvVars() {
	testCases := []struct {
		name string

		// Custom env variables to expose
		envVars map[string]string

		// Simulate that the crafting process is hapenning in a specific runner
		inGithubEnv  bool
		inJenkinsEnv bool

		expectedError   string
		expectedEnvVars map[string]string
	}{
		{
			name:          "missing custom vars",
			inGithubEnv:   true,
			expectedError: "required env variables not present \"CUSTOM_VAR_1\"",
		}, {
			name:          "missing some github runner env",
			inGithubEnv:   true,
			expectedError: "error while resolving runner environment variables: environment variable GITHUB_ACTOR cannot be resolved\n",
			envVars: map[string]string{
				"CUSTOM_VAR_1": "custom_value_1",
				"CUSTOM_VAR_2": "custom_value_2",
				"GITHUB_ACTOR": "", // This is removing one necessary variable
			},
		}, {
			name:          "missing optional jenkins variable with no error",
			inJenkinsEnv:  true,
			expectedError: "",
			envVars: map[string]string{
				// Missing var: GIT_BRANCH
				"CUSTOM_VAR_1": "custom_value_1",
				"CUSTOM_VAR_2": "custom_value_2",
			},
			expectedEnvVars: map[string]string{
				// Missing var: GIT_BRANCH
				"CUSTOM_VAR_1":  "custom_value_1",
				"CUSTOM_VAR_2":  "custom_value_2",
				"JOB_NAME":      "some-job",
				"BUILD_URL":     "http://some-url",
				"AGENT_WORKDIR": "/some/home/dir",
				"NODE_NAME":     "some-node",
			},
		}, {
			name:          "all optional jenkins variable with no error",
			inJenkinsEnv:  true,
			expectedError: "",
			envVars: map[string]string{
				"GIT_BRANCH":   "some-branch", // optional var 1
				"GIT_COMMIT":   "some-commit", // optional var 2
				"CUSTOM_VAR_1": "custom_value_1",
				"CUSTOM_VAR_2": "custom_value_2",
			},
			expectedEnvVars: map[string]string{
				"GIT_BRANCH":    "some-branch", // optional var 1
				"GIT_COMMIT":    "some-commit", // optional var 2
				"CUSTOM_VAR_1":  "custom_value_1",
				"CUSTOM_VAR_2":  "custom_value_2",
				"JOB_NAME":      "some-job",
				"BUILD_URL":     "http://some-url",
				"AGENT_WORKDIR": "/some/home/dir",
				"NODE_NAME":     "some-node",
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			var runner crafter.SupportedRunner = runners.NewGeneric()
			contract := "testdata/contracts/with_env_vars.yaml"
			if tc.inGithubEnv {
				s.T().Setenv("CI", "true")
				for k, v := range gitHubTestingEnvVars {
					s.T().Setenv(k, v)
				}
				runner = runners.NewGithubAction()
			} else if tc.inJenkinsEnv {
				contract = "testdata/contracts/jenkins_with_env_vars.yaml"
				s.T().Setenv("JOB_NAME", "some-job")
				s.T().Setenv("BUILD_URL", "http://some-url")
				s.T().Setenv("AGENT_WORKDIR", "/some/home/dir")
				s.T().Setenv("NODE_NAME", "some-node")
				s.T().Setenv("JENKINS_HOME", "/some/home/dir")
				runner = runners.NewJenkinsJob()
			}

			// Customs env vars
			for k, v := range tc.envVars {
				s.T().Setenv(k, v)
			}

			c, err := newInitializedCrafter(s.T(), contract, &v1.WorkflowMetadata{}, false, "", runner)
			require.NoError(s.T(), err)

			err = c.ResolveEnvVars(context.Background(), "")

			if tc.expectedError != "" {
				s.Error(err)
				actualError := err.Error()
				s.Equal(tc.expectedError, actualError)
				return
			}

			s.NoError(err)
			s.Equal(tc.expectedEnvVars, c.CraftingState.Attestation.EnvVars)
		})
	}
}

var gitHubTestingEnvVars = map[string]string{
	"GITHUB_REPOSITORY":       "chainloop/chainloop",
	"GITHUB_RUN_ID":           "123",
	"GITHUB_ACTOR":            "chainloop",
	"GITHUB_REF":              "refs/heads/main",
	"GITHUB_REPOSITORY_OWNER": "chainloop",
	"GITHUB_SHA":              "1234567890",
	"RUNNER_NAME":             "chainloop-runner",
	"RUNNER_OS":               "linux",
}

func (s *crafterSuite) TestAlreadyInitialized() {
	s.T().Run("already initialized", func(t *testing.T) {
		statePath := fmt.Sprintf("%s/attestation.json", t.TempDir())
		_, err := os.Create(statePath)
		require.NoError(s.T(), err)
		// TODO: replace by a mock
		c, err := crafter.NewCrafter(testingStateManager(t, statePath))
		require.NoError(s.T(), err)
		s.True(c.AlreadyInitialized(context.Background(), ""))
	})

	s.T().Run("non existing", func(t *testing.T) {
		statePath := fmt.Sprintf("%s/attestation.json", t.TempDir())
		c, err := crafter.NewCrafter(testingStateManager(t, statePath))
		require.NoError(s.T(), err)
		s.False(c.AlreadyInitialized(context.Background(), ""))
	})
}

func (s *crafterSuite) SetupTest() {
	s.workflowMetadata = &v1.WorkflowMetadata{
		WorkflowId:     "workflow-id",
		Name:           "workflow-name",
		Project:        "project",
		Team:           "team",
		SchemaRevision: "1",
	}

	// we need to make sure that these env variables are not set
	// because these tests might be happening indeed inside a Github Action
	for k := range gitHubTestingEnvVars {
		s.T().Setenv(k, "")
	}

	s.T().Setenv("CI", "")

	s.repoPath = s.T().TempDir()
	repo, err := git.PlainInit(s.repoPath, false)
	require.NoError(s.T(), err)
	wt, err := repo.Worktree()
	require.NoError(s.T(), err)

	filename := filepath.Join(s.repoPath, "example-git-file")
	if err = os.WriteFile(filename, []byte("hello world!"), 0600); err != nil {
		require.NoError(s.T(), err)
	}

	_, err = wt.Add("example-git-file")
	require.NoError(s.T(), err)

	h, err := wt.Commit("test commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	})
	require.NoError(s.T(), err)

	s.repoHead = h.String()
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(crafterSuite))
}

func (s *crafterSuite) TestAddMaterialsAutomatic() {
	testCases := []struct {
		name           string
		materialPath   string
		expectedType   schemaapi.CraftingSchema_Material_MaterialType
		uploadArtifact bool
		wantErr        bool
	}{
		{
			name:         "sarif",
			materialPath: "./materials/testdata/report.sarif",
			expectedType: schemaapi.CraftingSchema_Material_SARIF,
		},
		{
			name:         "openvex",
			materialPath: "./materials/testdata/openvex_v0.2.0.json",
			expectedType: schemaapi.CraftingSchema_Material_OPENVEX,
		},
		{
			name:         "HELM CHART",
			materialPath: "./materials/testdata/valid-chart.tgz",
			expectedType: schemaapi.CraftingSchema_Material_HELM_CHART,
		},
		{
			name:         "junit",
			materialPath: "./materials/testdata/junit.xml",
			expectedType: schemaapi.CraftingSchema_Material_JUNIT_XML,
		},
		{
			name:         "artifact",
			materialPath: "./materials/testdata/missing-empty.tgz",
			expectedType: schemaapi.CraftingSchema_Material_ARTIFACT,
		},
		{
			name:         "artifact - invalid junit",
			materialPath: "./materials/testdata/junit-invalid.xml",
			expectedType: schemaapi.CraftingSchema_Material_ARTIFACT,
		},
		{
			name:         "artifact - random file",
			materialPath: "./materials/testdata/random.json",
			expectedType: schemaapi.CraftingSchema_Material_ARTIFACT,
		},
		{
			name:           "random string",
			materialPath:   "random-string",
			expectedType:   schemaapi.CraftingSchema_Material_STRING,
			uploadArtifact: true,
		},
		{
			name:           "file too large",
			materialPath:   "./materials/testdata/sbom.cyclonedx.json",
			expectedType:   schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
			wantErr:        true,
			uploadArtifact: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			var runner crafter.SupportedRunner = runners.NewGeneric()
			contract := "testdata/contracts/empty_generic.yaml"
			uploader := mUploader.NewUploader(s.T())

			if !tc.uploadArtifact {
				uploader.On("UploadFile", context.Background(), tc.materialPath).
					Return(&casclient.UpDownStatus{
						Digest:   "deadbeef",
						Filename: "simple.txt",
					}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}

			// Establishing a maximum size for the artifact to be uploaded to the CAS causes an error
			// if the value is exceeded
			if tc.wantErr {
				backend.MaxSize = 1
			}

			c, err := newInitializedCrafter(s.T(), contract, &v1.WorkflowMetadata{}, false, "", runner)
			require.NoError(s.T(), err)

			kind, err := c.AddMaterialContactFreeAutomatic(context.Background(), "random-id", tc.materialPath, backend, nil)
			if tc.wantErr {
				assert.ErrorIs(s.T(), err, materials.ErrBaseUploadAndCraft)
			} else {
				require.NoError(s.T(), err)
			}
			assert.Equal(s.T(), tc.expectedType.String(), kind.String())
		})
	}
}
