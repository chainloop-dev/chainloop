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
	"testing"

	"github.com/stretchr/testify/suite"
)

type teamCityPipelineSuite struct {
	suite.Suite
	runner *TeamCityPipeline
}

func (s *teamCityPipelineSuite) TestCheckEnv() {
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
			name: "missing TEAMCITY_PROJECT_NAME",
			env: map[string]string{
				"BUILD_URL": "http://some-build-url/",
			},
			want: false,
		},
		{
			name: "all present",
			env: map[string]string{
				"TEAMCITY_PROJECT_NAME": "some-project",
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			os.Unsetenv("BUILD_URL")
			os.Unsetenv("TEAMCITY_PROJECT_NAME")

			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			s.Equal(tc.want, s.runner.CheckEnv())
		})
	}
}

func (s *teamCityPipelineSuite) TestListEnvVars() {
	s.Equal([]*EnvVarDefinition{
		{"BUILD_URL", false},
		{"TEAMCITY_PROJECT_NAME", false},
		{"TEAMCITY_VERSION", true},
		{"BUILD_NUMBER", true},
		{"USER", true},
		{"TEAMCITY_GIT_VERSION", true},
		{"BUILD_VCS_NUMBER", true},
		{"HOME", true},
	}, s.runner.ListEnvVars())
}

func (s *teamCityPipelineSuite) TestResolveEnvVars() {
	// Test with all of the environment variables present
	for k, v := range teamCityPipelineTestingEnvVars {
		os.Setenv(k, v)
	}
	resolvedEnvVars, errors := s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Equal(teamCityPipelineTestingEnvVars, resolvedEnvVars)

	// Test with the optional environment variables unset
	os.Unsetenv("TEAMCITY_VERSION")
	os.Unsetenv("BUILD_NUMBER")
	os.Unsetenv("USER")
	os.Unsetenv("TEAMCITY_GIT_VERSION")
	os.Unsetenv("BUILD_VCS_NUMBER")
	os.Unsetenv("HOME")
	resolvedEnvVars, errors = s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Equal(requiredTeamCityPipelineTestingEnvVars, resolvedEnvVars)
}

func (s *teamCityPipelineSuite) TestRunURI() {
	s.Equal("http://some-build-url/", s.runner.RunURI())
}

func (s *teamCityPipelineSuite) TestRunnerName() {
	s.Equal("TEAMCITY_PIPELINE", s.runner.ID().String())
}

// Run before each test
func (s *teamCityPipelineSuite) SetupTest() {
	s.runner = NewTeamCityPipeline()
	t := s.T()
	for k, v := range teamCityPipelineTestingEnvVars {
		t.Setenv(k, v)
	}
}

var teamCityPipelineTestingEnvVars = map[string]string{
	"BUILD_URL":             "http://some-build-url/",
	"TEAMCITY_PROJECT_NAME": "some-project",
	"TEAMCITY_VERSION":      "2023.1",
	"BUILD_NUMBER":          "1234",
	"USER":                  "teamcity",
	"TEAMCITY_GIT_VERSION":  "git version 2.30.0",
	"BUILD_VCS_NUMBER":      "abcdef123456",
	"HOME":                  "/home/teamcity",
}

var requiredTeamCityPipelineTestingEnvVars = map[string]string{
	"BUILD_URL":             "http://some-build-url/",
	"TEAMCITY_PROJECT_NAME": "some-project",
}

// Run the tests
func TestTeamCityPipelineRunner(t *testing.T) {
	suite.Run(t, new(teamCityPipelineSuite))
}
