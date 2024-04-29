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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
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
			name:           "dagger runner",
			testInput:      v1.CraftingSchema_Runner_DAGGER_PIPELINE,
			expectedOutput: "Dagger Pipeline",
		}, {
			name:           "circleci runner",
			testInput:      v1.CraftingSchema_Runner_CIRCLECI_BUILD,
			expectedOutput: "CircleCI Build",
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

func TestTransformWorkflowRunStatus(t *testing.T) {
	testCases := []struct {
		name           string
		testInput      string
		expectedOutput pb.RunStatus
	}{
		{
			name:           "initialized status",
			testInput:      "INITIALIZED",
			expectedOutput: pb.RunStatus_RUN_STATUS_INITIALIZED,
		}, {
			name:           "succeeded status",
			testInput:      "SUCCEEDED",
			expectedOutput: pb.RunStatus_RUN_STATUS_SUCCEEDED,
		}, {
			name:           "failed status",
			testInput:      "FAILED",
			expectedOutput: pb.RunStatus_RUN_STATUS_FAILED,
		}, {
			name:           "expired status",
			testInput:      "EXPIRED",
			expectedOutput: pb.RunStatus_RUN_STATUS_EXPIRED,
		}, {
			name:           "cancelled status",
			testInput:      "CANCELLED",
			expectedOutput: pb.RunStatus_RUN_STATUS_CANCELLED,
		}, {
			name:           "unknown status",
			testInput:      "UNKNOWN",
			expectedOutput: pb.RunStatus_RUN_STATUS_UNSPECIFIED,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := transformWorkflowRunStatus(testCase.testInput)
			if result != testCase.expectedOutput {
				t.Errorf("Expected %v, got %v", testCase.expectedOutput, result)
			}
		})
	}
}
