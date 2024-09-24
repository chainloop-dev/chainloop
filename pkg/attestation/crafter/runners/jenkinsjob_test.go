//
// Copyright 2024 The Chainloop Authors.
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

type jenkinsJobSuite struct {
	suite.Suite
	runner *JenkinsJob
}

func (s *jenkinsJobSuite) TestCheckEnv() {
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
			name: "missing JENKINS_HOME",
			env: map[string]string{
				"BUILD_URL": "http://some-build-url/",
			},
			want: false,
		},
		{
			name: "missing BUILD_URL",
			env: map[string]string{
				"JENKINS_HOME": "http://some-jenkins-home/",
			},
			want: false,
		},
		{
			name: "all present",
			env: map[string]string{
				"BUILD_URL":    "http://some-build-url/",
				"JENKINS_HOME": "http://some-jenkins-home/",
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			os.Unsetenv("BUILD_URL")
			os.Unsetenv("JENKINS_HOME")

			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			s.Equal(tc.want, s.runner.CheckEnv())
		})
	}
}

func (s *jenkinsJobSuite) TestListEnvVars() {
	s.Equal([]*EnvVarDefinition{
		{"JOB_NAME", false},
		{"BUILD_URL", false},
		{"GIT_BRANCH", true},
		{"GIT_COMMIT", true},
		{"AGENT_WORKDIR", true},
		{"WORKSPACE", false},
		{"NODE_NAME", false},
	}, s.runner.ListEnvVars())
}

func (s *jenkinsJobSuite) TestResolveEnvVars() {
	// Test with all of the environment variables present
	resolvedEnvVars, errors := s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Equal(jenkinsJobTestingEnvVars, resolvedEnvVars)

	// Test with the optional environment variables unset
	os.Unsetenv("GIT_BRANCH")
	os.Unsetenv("GIT_COMMIT")
	resolvedEnvVars, errors = s.runner.ResolveEnvVars()
	s.Empty(errors)
	s.Equal(requiredJenkinsJobTestingEnvVars, resolvedEnvVars)
}

func (s *jenkinsJobSuite) TestRunURI() {
	s.Equal("http://some-build-url/", s.runner.RunURI())
}

func (s *jenkinsJobSuite) TestRunnerName() {
	s.Equal("JENKINS_JOB", s.runner.ID().String())
}

// Run before each test
func (s *jenkinsJobSuite) SetupTest() {
	s.runner = NewJenkinsJob()
	t := s.T()
	for k, v := range jenkinsJobTestingEnvVars {
		t.Setenv(k, v)
	}
}

var jenkinsJobTestingEnvVars = map[string]string{
	"JOB_NAME":   "some-jenkins-job",
	"BUILD_URL":  "http://some-build-url/",
	"WORKSPACE":  "/home/sample/agent",
	"NODE_NAME":  "some-node",
	"GIT_BRANCH": "somebranch",
	"GIT_COMMIT": "somecommit",
}

var requiredJenkinsJobTestingEnvVars = map[string]string{
	"JOB_NAME":  "some-jenkins-job",
	"BUILD_URL": "http://some-build-url/",
	"WORKSPACE": "/home/sample/agent",
	"NODE_NAME": "some-node",
}

// Run the tests
func TestJenkinsJobRunner(t *testing.T) {
	suite.Run(t, new(jenkinsJobSuite))
}
