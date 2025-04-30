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
		{"CI_PROJECT_URL", false},
		{"CI_COMMIT_SHA", false},
		{"CI_JOB_URL", false},
		{"CI_PIPELINE_URL", false},
		{"CI_RUNNER_VERSION", false},
		{"CI_RUNNER_DESCRIPTION", false},
		{"CI_COMMIT_REF_NAME", false},
	}, s.runner.ListEnvVars())
}

func (s *gitlabPipelineSuite) TestResolveEnvVars() {
	resolvedEnvVars, errors := s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Equal(map[string]string{
		"GITLAB_USER_EMAIL":     "foo@foo.com",
		"GITLAB_USER_LOGIN":     "foo",
		"CI_PROJECT_URL":        "https://gitlab.com/chainloop/chainloop",
		"CI_COMMIT_SHA":         "1234567890",
		"CI_JOB_URL":            "https://gitlab.com/chainloop/chainloop/-/jobs/123",
		"CI_PIPELINE_URL":       "https://gitlab.com/chainloop/chainloop/-/pipelines/123",
		"CI_RUNNER_VERSION":     "13.10.0",
		"CI_RUNNER_DESCRIPTION": "chainloop-runner",
		"CI_COMMIT_REF_NAME":    "main",
	}, resolvedEnvVars)
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
	s.runner = NewGitlabPipeline(context.Background(), &logger)
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
}

// Run the tests
func TestGitlabPipelineRunner(t *testing.T) {
	suite.Run(t, new(gitlabPipelineSuite))
}
