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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type azurePipelineSuite struct {
	suite.Suite
	runner *AzurePipeline
}

func (s *azurePipelineSuite) TestCheckEnv() {
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
				"TF_BUILD": "true",
			},
			want: false,
		},
		{
			name: "all present",
			env: map[string]string{
				"TF_BUILD":       "true",
				"BUILD_BUILDURI": "chainloop/chainloop",
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			os.Unsetenv("TF_BUILD")
			os.Unsetenv("BUILD_BUILDURI")

			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			s.Equal(tc.want, s.runner.CheckEnv())
		})
	}
}

func (s *azurePipelineSuite) TestListEnvVars() {
	assert.Equal(s.T(), []*EnvVarDefinition{
		{"BUILD_REQUESTEDFOREMAIL", false},
		{"BUILD_REQUESTEDFOR", false},
		{"BUILD_REPOSITORY_URI", false},
		{"BUILD_REPOSITORY_NAME", false},
		{"BUILD_BUILDID", false},
		{"BUILD_BUILDNUMBER", false},
		{"BUILD_BUILDURI", false},
		{"BUILD_REASON", false},
		{"AGENT_VERSION", false},
		{"TF_BUILD", false},
	}, s.runner.ListEnvVars())
}

func (s *azurePipelineSuite) TestResolveEnvVars() {
	resolvedEnvVars, err := s.runner.ResolveEnvVars()
	s.NoError(err)

	s.Equal(map[string]string{
		"AGENT_VERSION":           "3.220.5",
		"BUILD_BUILDID":           "6",
		"BUILD_BUILDNUMBER":       "20230726.5",
		"BUILD_BUILDURI":          "vstfs:///Build/Build/6",
		"BUILD_REASON":            "IndividualCI",
		"BUILD_REPOSITORY_NAME":   "chainloop-tests",
		"BUILD_REPOSITORY_URI":    "https://chainlooptest@dev.azure.com/chainloop-test/chainloop-tests/_git/chainloop-tests",
		"BUILD_REQUESTEDFOR":      "Jan Kowalsky",
		"BUILD_REQUESTEDFOREMAIL": "jan@kowalscy.onmicrosoft.com",
		"TF_BUILD":                "True",
	}, resolvedEnvVars)
}

func (s *azurePipelineSuite) TestRunURI() {
	s.Equal("https://dev.azure.com/chainloop-test/chainloop-tests/_build/results?buildId=6&j=12f1170f-0000-0000-20dd-22fc7dff55f9&view=logs", s.runner.RunURI())
}

func (s *azurePipelineSuite) TestRunnerName() {
	s.Equal("azure-pipeline", s.runner.String())
}

// Run before each test
func (s *azurePipelineSuite) SetupTest() {
	s.runner = NewAzurePipeline()
	t := s.T()
	t.Setenv("TF_BUILD", "True")
	t.Setenv("BUILD_REPOSITORY_ID", "5e5bf8eb-0000-0000-801b-0a5bc4b4011a")
	t.Setenv("BUILD_REPOSITORY_URI", "https://chainlooptest@dev.azure.com/chainloop-test/chainloop-tests/_git/chainloop-tests")
	t.Setenv("BUILD_REPOSITORY_NAME", "chainloop-tests")
	t.Setenv("BUILD_SOURCEVERSIONAUTHOR", "Jan Kowalsky")
	t.Setenv("BUILD_REQUESTEDFOR", "Jan Kowalsky")
	t.Setenv("BUILD_REQUESTEDFOREMAIL", "jan@kowalscy.onmicrosoft.com")
	t.Setenv("BUILD_SOURCEVERSION", "612a6f172be5fcca249b02ae0c3bbab09d59a0f5")
	t.Setenv("BUILD_BUILDID", "6")
	t.Setenv("BUILD_BUILDNUMBER", "20230726.5")
	t.Setenv("BUILD_BUILDURI", "vstfs:///Build/Build/6")
	t.Setenv("BUILD_CONTAINERID", "170183")
	t.Setenv("ENDPOINT_URL_SYSTEMVSSCONNECTION", "https://dev.azure.com/chainloop-test/")
	t.Setenv("BUILD_REASON", "IndividualCI")
	t.Setenv("AGENT_VERSION", "3.220.5")
	t.Setenv("SYSTEM_COLLECTIONID", "e2dadf5b-9a6d-0000-0000-89ad0786f16e")
	t.Setenv("SYSTEM_TEAMPROJECTID", "e0730109-da00-0000-0000-80abab2033a2")
	t.Setenv("SYSTEM_TEAMPROJECT", "chainloop-tests")
	t.Setenv("SYSTEM_TEAMFOUNDATIONSERVERURI", "https://dev.azure.com/chainloop-test/")
	t.Setenv("SYSTEM_DEFINITIONNAME", "chainloop-tests")
	t.Setenv("SYSTEM_STAGEID", "96ac2280-0000-0000-99de-dd2da759617d")
	t.Setenv("BUILD_REQUESTEDFORID", "4962d626-0000-0000-ae45-95ea268aa3e8")
	t.Setenv("SYSTEM_JOBID", "12f1170f-0000-0000-20dd-22fc7dff55f9")
	t.Setenv("AGENT_ID", "9")
	t.Setenv("SYSTEM_ISAZUREVM", "0")
	t.Setenv("SYSTEM_TASKINSTANCEID", "f8ed7bd8-0000-0000-9385-7fc29a8b5b7b")
}

// Run the tests
func TestAzurePipelineRunner(t *testing.T) {
	suite.Run(t, new(azurePipelineSuite))
}
