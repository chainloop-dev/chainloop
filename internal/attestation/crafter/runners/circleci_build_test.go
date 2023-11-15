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
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type circleCIBuildSuite struct {
	suite.Suite
	runner *CircleCIBuild
}

func (s *circleCIBuildSuite) TestCheckEnv() {
	testCases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			name: "empty",
			env:  map[string]string{},
			want: false,
		}, {
			name: "all present",
			env: map[string]string{
				"CI":       "true",
				"CIRCLECI": "true",
			},
			want: true,
		}, {
			name: "missing CI",
			env: map[string]string{
				"CIRCLECI": "true",
			},
			want: false,
		}, {
			name: "all present but CI false",
			env: map[string]string{
				"CI":       "false",
				"CIRCLECI": "true",
			},
			want: false,
		}, {
			name: "missing CIECLECI",
			env: map[string]string{
				"CI": "true",
			},
			want: false,
		}, {
			name: "all present but CIRCLECI false",
			env: map[string]string{
				"CI":       "true",
				"CIRCLECI": "false",
			},
			want: false,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			os.Unsetenv("CI")
			os.Unsetenv("CIRCLECI")

			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			s.Equal(tc.want, s.runner.CheckEnv())
		})
	}
}

func (s *circleCIBuildSuite) TestListEnvVars() {
	s.Equal([]string{
		"CIRCLE_BUILD_URL",
		"CIRCLE_JOB",
		"CIRCLE_BRANCH",
		"CIRCLE_NODE_TOTAL",
		"CIRCLE_NODE_INDEX",
	}, s.runner.ListEnvVars())
}

func (s *circleCIBuildSuite) TestResolveEnvVars() {
	s.Equal(circleCIBuildTestingEnvVars, s.runner.ResolveEnvVars())
}

func (s *circleCIBuildSuite) TestRunURI() {
	s.Equal("http://some-build-url/", s.runner.RunURI())
}

func (s *circleCIBuildSuite) TestRunnerName() {
	s.Equal("jenkins-job", s.runner.String())
}

// Run before each test
func (s *circleCIBuildSuite) SetupTest() {
	s.runner = NewCircleCIBuild()
	t := s.T()
	for k, v := range circleCIBuildTestingEnvVars {
		t.Setenv(k, v)
	}
}

var circleCIBuildTestingEnvVars = map[string]string{
	"CIRCLE_BUILD_URL":  "http://some-build-url/",
	"CIRCLE_JOB":        "some-job",
	"CIRCLE_BRANCH":     "some-branch",
	"CIRCLE_NODE_TOTAL": "3",
	"CIRCLE_NODE_INDEX": "1",
	"CIRCLE_PR_NUMBER":  "1337",
}

// Run the tests
func TestCircleCIBuildRunner(t *testing.T) {
	suite.Run(t, new(circleCIBuildSuite))
}
