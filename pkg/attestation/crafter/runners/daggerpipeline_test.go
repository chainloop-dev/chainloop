//
// Copyright 2024-2026 The Chainloop Authors.
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type daggerPipelineSuite struct {
	suite.Suite
	runner *DaggerPipeline
}

func (s *daggerPipelineSuite) TestCheckEnv() {
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
			name: "present",
			env: map[string]string{
				"CHAINLOOP_DAGGER_CLIENT": "v1.0.0",
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			os.Unsetenv("CHAINLOOP_DAGGER_CLIENT")

			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			s.Equal(tc.want, s.runner.CheckEnv())
		})
	}
}

func (s *daggerPipelineSuite) TestListEnvVars() {
	expected := []*EnvVarDefinition{
		{"CHAINLOOP_DAGGER_CLIENT", false},
		// Github Actions PR-specific variables
		{"GITHUB_EVENT_NAME", true},
		{"GITHUB_HEAD_REF", true},
		{"GITHUB_BASE_REF", true},
		{"GITHUB_EVENT_PATH", true},
		// Gitlab CI MR-specific variables
		{"CI_PIPELINE_SOURCE", true},
		{"CI_MERGE_REQUEST_IID", true},
		{"CI_MERGE_REQUEST_TITLE", true},
		{"CI_MERGE_REQUEST_DESCRIPTION", true},
		{"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", true},
		{"CI_MERGE_REQUEST_TARGET_BRANCH_NAME", true},
		{"CI_MERGE_REQUEST_PROJECT_URL", true},
		{"GITLAB_USER_LOGIN", true},
	}
	assert.Equal(s.T(), expected, s.runner.ListEnvVars())
}

func (s *daggerPipelineSuite) TestResolveEnvVars() {
	resolvedEnvVars, errors := s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Equal(map[string]string{"CHAINLOOP_DAGGER_CLIENT": "v0.6.0"}, resolvedEnvVars)
}

func (s *daggerPipelineSuite) TestResolveEnvVarsWithGithubPRContext() {
	t := s.T()
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	t.Setenv("GITHUB_HEAD_REF", "feature-branch")
	t.Setenv("GITHUB_BASE_REF", "main")
	t.Setenv("GITHUB_EVENT_PATH", "/tmp/github_event.json")

	resolvedEnvVars, errors := s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Equal(map[string]string{
		"CHAINLOOP_DAGGER_CLIENT": "v0.6.0",
		"GITHUB_EVENT_NAME":       "pull_request",
		"GITHUB_HEAD_REF":         "feature-branch",
		"GITHUB_BASE_REF":         "main",
		"GITHUB_EVENT_PATH":       "/tmp/github_event.json",
	}, resolvedEnvVars)
}

func (s *daggerPipelineSuite) TestResolveEnvVarsWithGitlabMRContext() {
	t := s.T()
	t.Setenv("CI_PIPELINE_SOURCE", "merge_request_event")
	t.Setenv("CI_MERGE_REQUEST_IID", "123")
	t.Setenv("CI_MERGE_REQUEST_TITLE", "Test MR")
	t.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", "feature-branch")
	t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")

	resolvedEnvVars, errors := s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Equal(map[string]string{
		"CHAINLOOP_DAGGER_CLIENT":             "v0.6.0",
		"CI_PIPELINE_SOURCE":                  "merge_request_event",
		"CI_MERGE_REQUEST_IID":                "123",
		"CI_MERGE_REQUEST_TITLE":              "Test MR",
		"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "feature-branch",
		"CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "main",
	}, resolvedEnvVars)
}

func (s *daggerPipelineSuite) TestRunURI() {
	s.Equal("", s.runner.RunURI())
}

func (s *daggerPipelineSuite) TestRunnerName() {
	s.Equal("DAGGER_PIPELINE", s.runner.ID().String())
}

// Run before each test
func (s *daggerPipelineSuite) SetupTest() {
	s.runner = NewDaggerPipeline()
	t := s.T()
	t.Setenv("CHAINLOOP_DAGGER_CLIENT", "v0.6.0")
}

// Run the tests
func TestDaggerPipelineRunner(t *testing.T) {
	suite.Run(t, new(daggerPipelineSuite))
}
