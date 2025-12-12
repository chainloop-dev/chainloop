//
// Copyright 2023-2025 The Chainloop Authors.
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
	"context"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type gitlabPipelineSuite struct {
	suite.Suite
	runner *GitlabPipeline
}

func (s *gitlabPipelineSuite) TestCheckEnv() {
	testCases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			name: "empty",
			env:  map[string]string{},
			want: false,
		},
		{
			name: "missing CI",
			env: map[string]string{
				"CI_JOB_URL": "chainloop/chainloop",
			},
			want: false,
		},
		{
			name: "missing JOB_URL",
			env: map[string]string{
				"GITLAB_CI": "true",
			},
			want: false,
		},
		{
			name: "all present",
			env: map[string]string{
				"GITLAB_CI":  "true",
				"CI_JOB_URL": "chainloop/chainloop",
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			os.Unsetenv("GITLAB_CI")
			os.Unsetenv("CI_JOB_URL")

			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			s.Equal(tc.want, s.runner.CheckEnv())
		})
	}
}

func (s *gitlabPipelineSuite) TestListEnvVars() {
	assert.Equal(s.T(), []*EnvVarDefinition{
		{"GITLAB_USER_EMAIL", false},
		{"GITLAB_USER_LOGIN", false},
		{"CI_SERVER_URL", false},
		{"CI_PROJECT_URL", false},
		{"CI_COMMIT_SHA", false},
		{"CI_JOB_URL", false},
		{"CI_PIPELINE_URL", false},
		{"CI_RUNNER_VERSION", false},
		{"CI_RUNNER_DESCRIPTION", true},
		{"CI_COMMIT_REF_NAME", false},
		{"CI_PIPELINE_SOURCE", true},
		{"CI_MERGE_REQUEST_IID", true},
		{"CI_MERGE_REQUEST_TITLE", true},
		{"CI_MERGE_REQUEST_DESCRIPTION", true},
		{"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", true},
		{"CI_MERGE_REQUEST_TARGET_BRANCH_NAME", true},
		{"CI_MERGE_REQUEST_PROJECT_URL", true},
	}, s.runner.ListEnvVars())
}

func (s *gitlabPipelineSuite) TestResolveEnvVars() {
	resolvedEnvVars, errors := s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Equal(map[string]string{
		"GITLAB_USER_EMAIL":                   "foo@foo.com",
		"GITLAB_USER_LOGIN":                   "foo",
		"CI_PROJECT_URL":                      "https://gitlab.com/chainloop/chainloop",
		"CI_COMMIT_SHA":                       "1234567890",
		"CI_JOB_URL":                          "https://gitlab.com/chainloop/chainloop/-/jobs/123",
		"CI_PIPELINE_URL":                     "https://gitlab.com/chainloop/chainloop/-/pipelines/123",
		"CI_RUNNER_VERSION":                   "13.10.0",
		"CI_RUNNER_DESCRIPTION":               "chainloop-runner",
		"CI_COMMIT_REF_NAME":                  "main",
		"CI_SERVER_URL":                       "https://gitlab.com",
		"CI_PIPELINE_SOURCE":                  "merge_request_event",
		"CI_MERGE_REQUEST_IID":                "42",
		"CI_MERGE_REQUEST_TITLE":              "Add new feature",
		"CI_MERGE_REQUEST_DESCRIPTION":        "Implements awesome feature",
		"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "feature/awesome",
		"CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "main",
		"CI_MERGE_REQUEST_PROJECT_URL":        "https://gitlab.com/chainloop/chainloop/-/merge_requests/42",
	}, resolvedEnvVars)
}

func (s *gitlabPipelineSuite) TestResolveEnvVarsWithoutRunnerDescription() {
	// Unset optional variables to test they can be missing
	s.T().Setenv("CI_RUNNER_DESCRIPTION", "")
	s.T().Setenv("CI_PIPELINE_SOURCE", "")
	s.T().Setenv("CI_MERGE_REQUEST_IID", "")
	s.T().Setenv("CI_MERGE_REQUEST_TITLE", "")
	s.T().Setenv("CI_MERGE_REQUEST_DESCRIPTION", "")
	s.T().Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", "")
	s.T().Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "")
	s.T().Setenv("CI_MERGE_REQUEST_PROJECT_URL", "")

	resolvedEnvVars, errors := s.runner.ResolveEnvVars()
	s.Empty(errors, "Should not error when optional variables are missing")

	expected := map[string]string{
		"GITLAB_USER_EMAIL":  "foo@foo.com",
		"GITLAB_USER_LOGIN":  "foo",
		"CI_PROJECT_URL":     "https://gitlab.com/chainloop/chainloop",
		"CI_COMMIT_SHA":      "1234567890",
		"CI_JOB_URL":         "https://gitlab.com/chainloop/chainloop/-/jobs/123",
		"CI_PIPELINE_URL":    "https://gitlab.com/chainloop/chainloop/-/pipelines/123",
		"CI_RUNNER_VERSION":  "13.10.0",
		"CI_COMMIT_REF_NAME": "main",
		"CI_SERVER_URL":      "https://gitlab.com",
	}
	s.Equal(expected, resolvedEnvVars)
}

func (s *gitlabPipelineSuite) TestRunURI() {
	s.Equal("https://gitlab.com/chainloop/chainloop/-/jobs/123", s.runner.RunURI())
}

func (s *gitlabPipelineSuite) TestRunnerName() {
	s.Equal("GITLAB_PIPELINE", s.runner.ID().String())
}

// Run before each test
func (s *gitlabPipelineSuite) SetupTest() {
	logger := zerolog.New(zerolog.Nop()).Level(zerolog.Disabled)
	s.runner = NewGitlabPipeline(context.Background(), "test-token", &logger)
	t := s.T()
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("GITLAB_USER_EMAIL", "foo@foo.com")
	t.Setenv("GITLAB_USER_LOGIN", "foo")
	t.Setenv("CI_PROJECT_URL", "https://gitlab.com/chainloop/chainloop")
	t.Setenv("CI_COMMIT_SHA", "1234567890")
	t.Setenv("CI_JOB_URL", "https://gitlab.com/chainloop/chainloop/-/jobs/123")
	t.Setenv("CI_PIPELINE_URL", "https://gitlab.com/chainloop/chainloop/-/pipelines/123")
	t.Setenv("CI_RUNNER_VERSION", "13.10.0")
	t.Setenv("CI_RUNNER_DESCRIPTION", "chainloop-runner")
	t.Setenv("CI_COMMIT_REF_NAME", "main")
	t.Setenv("CI_SERVER_URL", "https://gitlab.com")
	t.Setenv("CI_PIPELINE_SOURCE", "merge_request_event")
	t.Setenv("CI_MERGE_REQUEST_IID", "42")
	t.Setenv("CI_MERGE_REQUEST_TITLE", "Add new feature")
	t.Setenv("CI_MERGE_REQUEST_DESCRIPTION", "Implements awesome feature")
	t.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", "feature/awesome")
	t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")
	t.Setenv("CI_MERGE_REQUEST_PROJECT_URL", "https://gitlab.com/chainloop/chainloop/-/merge_requests/42")
}

// Run the tests
func TestGitlabPipelineRunner(t *testing.T) {
	suite.Run(t, new(gitlabPipelineSuite))
}
