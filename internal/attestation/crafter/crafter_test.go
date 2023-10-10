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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/suite"
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
			dryRun:           true,
		},
		{
			name:             "missing metadata",
			contractPath:     "testdata/contracts/empty_generic.yaml",
			workflowMetadata: nil,
			wantErr:          true,
		},
		{
			name:             "required github action environment, can't run",
			contractPath:     "testdata/contracts/empty_github.yaml",
			workflowMetadata: s.workflowMetadata,
			wantErr:          true,
		},
		{
			name:             "required github action env (dry run)",
			contractPath:     "testdata/contracts/empty_github.yaml",
			workflowMetadata: s.workflowMetadata,
			wantErr:          false,
			dryRun:           true,
		},
		{
			name:             "with annotations",
			contractPath:     "testdata/contracts/with_material_annotations.yaml",
			workflowMetadata: s.workflowMetadata,
			dryRun:           true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			contract, err := crafter.LoadSchema(tc.contractPath)
			require.NoError(s.T(), err)

			// Make sure that the tests context indicate that we are not in a CI
			// this makes the github action runner context to fail
			c, err := newInitializedCrafter(s.T(), tc.contractPath, tc.workflowMetadata, tc.dryRun, tc.workingDir)
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
				want.Attestation.Sha1Commit = s.repoHead
			}

			// reset to nil to easily compare them
			s.NotNil(c.CraftingState.Attestation.InitializedAt)
			c.CraftingState.Attestation.InitializedAt = nil
			c.CraftingState.Attestation.RunnerUrl = ""

			// Check state
			if ok := proto.Equal(want, c.CraftingState); !ok {
				s.Fail(fmt.Sprintf("These two protobuf messages are not equal:\nexpected: %v\nactual:  %v", want, c.CraftingState))
			}

			// Check state file
			_, err = os.Stat(c.statePath)
			s.NoError(err, "state file should be created")
		})
	}
}

type testingCrafter struct {
	*crafter.Crafter
	statePath string
}

func newInitializedCrafter(t *testing.T, contractPath string, wfMeta *v1.WorkflowMetadata, dryRun bool, workingDir string) (*testingCrafter, error) {
	contract, err := crafter.LoadSchema(contractPath)
	if err != nil {
		return nil, err
	}

	statePath := fmt.Sprintf("%s/attestation.json", t.TempDir())
	opts := []crafter.NewOpt{crafter.WithStatePath(statePath)}
	if workingDir != "" {
		opts = append(opts, crafter.WithWorkingDirPath(workingDir))
	}

	c := crafter.NewCrafter(opts...)
	if err = c.Init(&crafter.InitOpts{SchemaV1: contract, WfInfo: wfMeta, DryRun: dryRun}); err != nil {
		return nil, err
	}

	return &testingCrafter{c, statePath}, nil
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
		name   string
		strict bool
		// Custom env variables to expose
		envVars map[string]string
		// Simulate that the crafting process is hapenning in a github action runner
		inGithubEnv bool
		wantErr     bool
		// Total list of resolved env vars
		want map[string]string
	}{
		{
			name:        "strict missing custom vars",
			strict:      true,
			inGithubEnv: true,
			wantErr:     true,
		},
		{
			name:        "strict, not running in github env",
			strict:      true,
			inGithubEnv: false,
			wantErr:     true,
			envVars: map[string]string{
				"CUSTOM_VAR_1": "custom_value_1",
				"CUSTOM_VAR_2": "custom_value_2",
			},
		},
		{
			name:        "strict, missing some github env",
			strict:      true,
			inGithubEnv: false,
			wantErr:     true,
			envVars: map[string]string{
				"CUSTOM_VAR_1":  "custom_value_1",
				"CUSTOM_VAR_2":  "custom_value_2",
				"CI":            "true",
				"GITHUB_RUN_ID": "123",
			},
		},
		{
			name:        "strict and all envs available",
			strict:      true,
			inGithubEnv: true,
			envVars: map[string]string{
				"CUSTOM_VAR_1": "custom_value_1",
				"CUSTOM_VAR_2": "custom_value_2",
			},
			want: map[string]string{
				"CUSTOM_VAR_1":            "custom_value_1",
				"CUSTOM_VAR_2":            "custom_value_2",
				"GITHUB_REPOSITORY":       "chainloop/chainloop",
				"GITHUB_RUN_ID":           "123",
				"GITHUB_ACTOR":            "chainloop",
				"GITHUB_REF":              "refs/heads/main",
				"GITHUB_REPOSITORY_OWNER": "chainloop",
				"GITHUB_SHA":              "1234567890",
				"RUNNER_NAME":             "chainloop-runner",
				"RUNNER_OS":               "linux",
			},
		},
		{
			name:        "non strict missing custom vars",
			strict:      false,
			inGithubEnv: true,
			wantErr:     false,
			want:        gitHubTestingEnvVars,
		},
		{
			name:        "non strict, missing some github env",
			strict:      false,
			inGithubEnv: false,
			wantErr:     false,
			envVars: map[string]string{
				"CUSTOM_VAR_1":      "custom_value_1",
				"CUSTOM_VAR_2":      "custom_value_2",
				"CI":                "true",
				"GITHUB_RUN_ID":     "123",
				"GITHUB_REPOSITORY": "chainloop/chainloop",
			},
			want: map[string]string{
				"CUSTOM_VAR_1":            "custom_value_1",
				"CUSTOM_VAR_2":            "custom_value_2",
				"GITHUB_REPOSITORY":       "chainloop/chainloop",
				"GITHUB_RUN_ID":           "123",
				"GITHUB_ACTOR":            "",
				"GITHUB_REF":              "",
				"GITHUB_REPOSITORY_OWNER": "",
				"GITHUB_SHA":              "",
				"RUNNER_NAME":             "",
				"RUNNER_OS":               "",
			},
		},
		{
			name:        "non strict, wrong runner context",
			strict:      false,
			inGithubEnv: false,
			wantErr:     false,
			envVars: map[string]string{
				"CUSTOM_VAR_1": "custom_value_1",
				"CUSTOM_VAR_2": "custom_value_2",
			},
			want: map[string]string{
				"CUSTOM_VAR_1": "custom_value_1",
				"CUSTOM_VAR_2": "custom_value_2",
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Customs env vars
			for k, v := range tc.envVars {
				s.T().Setenv(k, v)
			}
			// Runner env variables
			if tc.inGithubEnv {
				s.T().Setenv("CI", "true")
				for k, v := range gitHubTestingEnvVars {
					s.T().Setenv(k, v)
				}
			}

			c, err := newInitializedCrafter(s.T(), "testdata/contracts/with_env_vars.yaml", &v1.WorkflowMetadata{}, true, "")
			require.NoError(s.T(), err)

			err = c.ResolveEnvVars(tc.strict)
			if tc.wantErr {
				s.Error(err)
				return
			}

			s.NoError(err)
			s.Equal(tc.want, c.CraftingState.Attestation.EnvVars)
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
		c := crafter.NewCrafter(crafter.WithStatePath(statePath))
		s.True(c.AlreadyInitialized())
	})

	s.T().Run("non existing", func(t *testing.T) {
		statePath := fmt.Sprintf("%s/attestation.json", t.TempDir())
		c := crafter.NewCrafter(crafter.WithStatePath(statePath))
		s.False(c.AlreadyInitialized())
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

	h, err := wt.Commit("test commit", &git.CommitOptions{})
	require.NoError(s.T(), err)

	s.repoHead = h.String()
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(crafterSuite))
}
