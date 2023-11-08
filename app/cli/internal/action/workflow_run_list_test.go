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

package action

import (
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/stretchr/testify/suite"
)

type workflowRunListSuite struct {
	suite.Suite
}

func (s *workflowRunListSuite) TestHumanizedRunnerType() {
	testCases := []struct {
		name           string
		testInput      v1.CraftingSchema_Runner_RunnerType
		expectedOutput string
	}{
		{
			name:           "unspecified runner",
			testInput:      v1.CraftingSchema_Runner_RUNNER_TYPE_UNSPECIFIED,
			expectedOutput: "Unspecified",
		}, {
			name:           "github runner",
			testInput:      v1.CraftingSchema_Runner_GITHUB_ACTION,
			expectedOutput: "GitHub",
		}, {
			name:           "gitlab runner",
			testInput:      v1.CraftingSchema_Runner_GITLAB_PIPELINE,
			expectedOutput: "GitLab",
		}, {
			name:           "azure runner",
			testInput:      v1.CraftingSchema_Runner_AZURE_PIPELINE,
			expectedOutput: "Azure Pipeline",
		}, {
			name:           "jenkins runner",
			testInput:      v1.CraftingSchema_Runner_JENKINS_JOB,
			expectedOutput: "Jenkins Job",
		}, {
			name:           "unknown runner",
			testInput:      -34,
			expectedOutput: "Unknown",
		},
	}

	// enforce 1 test case per runner (+ the unknown)
	nRunnerTypes := len(v1.CraftingSchema_Runner_RunnerType_name)
	nTestCases := len(testCases)
	s.Equal(
		nTestCases-1,
		nRunnerTypes,
		"%d runners detected vs. %d test entries",
		nRunnerTypes,
		nTestCases-1,
	)

	for _, testCase := range testCases {
		s.T().Run(testCase.name, func(t *testing.T) {
			result := humanizedRunnerType(testCase.testInput)
			s.Equal(testCase.expectedOutput, result)
		})
	}
}

func TestWorkflowRunlist(t *testing.T) {
	suite.Run(t, new(workflowRunListSuite))
}
