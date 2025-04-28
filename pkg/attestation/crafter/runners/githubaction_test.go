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

	"github.com/stretchr/testify/suite"
)

type githubActionSuite struct {
	suite.Suite
	runner *GitHubAction
}

func (s *githubActionSuite) TestCheckEnv() {
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
				"GITHUB_REPOSITORY": "chainloop/chainloop",
				"GITHUB_RUN_ID":     "123",
			},
			want: false,
		},
		{
			name: "missing GITHUB_REPOSITORY",
			env: map[string]string{
				"CI":            "true",
				"GITHUB_RUN_ID": "123",
			},
			want: false,
		},
		{
			name: "missing GITHUB_RUN_ID",
			env: map[string]string{
				"CI":                "true",
				"GITHUB_REPOSITORY": "chainloop/chainloop",
			},
			want: false,
		},
		{
			name: "all present",
			env: map[string]string{
				"CI":                "true",
				"GITHUB_REPOSITORY": "chainloop/chainloop",
				"GITHUB_RUN_ID":     "123",
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			os.Unsetenv("CI")
			os.Unsetenv("GITHUB_REPOSITORY")
			os.Unsetenv("GITHUB_RUN_ID")

			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			s.Equal(tc.want, s.runner.CheckEnv())
		})
	}
}

func (s *githubActionSuite) TestListEnvVars() {
	s.Equal([]*EnvVarDefinition{
		{"GITHUB_ACTOR", false},
		{"GITHUB_REF", false},
		{"GITHUB_REPOSITORY", false},
		{"GITHUB_REPOSITORY_OWNER", false},
		{"GITHUB_RUN_ID", false},
		{"GITHUB_SHA", false},
		{"RUNNER_NAME", false},
		{"RUNNER_OS", false},
	}, s.runner.ListEnvVars())
}

func (s *githubActionSuite) TestResolveEnvVars() {
	resolvedEnvVars, errors := s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Equal(gitHubTestingEnvVars, resolvedEnvVars)
}

func (s *githubActionSuite) TestRunURI() {
	s.Equal("https://github.com/chainloop/chainloop/actions/runs/123", s.runner.RunURI())
}

func (s *githubActionSuite) TestRunnerName() {
	s.Equal("GITHUB_ACTION", s.runner.ID().String())
}

// Run before each test
func (s *githubActionSuite) SetupTest() {
	// Create a logger for testing
	testLogger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel)
	s.runner = NewGithubAction(context.Background(), &testLogger)
	t := s.T()
	for k, v := range gitHubTestingEnvVars {
		t.Setenv(k, v)
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

// Run the tests
func TestGithubActionRunner(t *testing.T) {
	suite.Run(t, new(githubActionSuite))
}
