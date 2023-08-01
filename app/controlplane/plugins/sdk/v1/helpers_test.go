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

package sdk

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *helperTestSuite) TestSummaryTable() {
	testCases := []struct {
		name       string
		inputPath  string
		renderOpts []RenderOpt
		outputPath string
		hasError   bool
	}{
		{
			name:       "full text",
			inputPath:  "testdata/attestations/full.json",
			outputPath: "testdata/attestations/full.txt",
		},
		{
			name:       "truncated text",
			inputPath:  "testdata/attestations/full.json",
			renderOpts: []RenderOpt{WithMaxSize(2000)},
			outputPath: "testdata/attestations/truncated.txt",
		},
		{
			name:       "full markdown",
			inputPath:  "testdata/attestations/full.json",
			renderOpts: []RenderOpt{WithFormat("markdown")},
			outputPath: "testdata/attestations/full.md",
		},
		{
			name:       "invalid format",
			inputPath:  "testdata/attestations/full.json",
			renderOpts: []RenderOpt{WithFormat("invalid")},
			outputPath: "testdata/attestations/full.md",
			hasError:   true,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			renderer, err := newRenderer(tc.renderOpts...)
			if tc.hasError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			want, err := os.ReadFile(tc.outputPath)
			require.NoError(t, err)

			predicate, err := testPredicate(tc.inputPath)
			require.NoError(t, err)
			got, err := renderer.summaryTable(s.m, predicate)
			if tc.hasError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(s.T(), got, string(want))
		})
	}
}

func TestHelpers(t *testing.T) {
	suite.Run(t, new(helperTestSuite))
}

func (s *helperTestSuite) SetupTest() {
	date, _ := time.Parse("2006-01-02", "2021-11-22")
	s.m = &ChainloopMetadata{
		Workflow: &ChainloopMetadataWorkflow{
			ID:      "deadbeef",
			Name:    "test-workflow",
			Project: "test-project",
			Team:    "test-team",
		},
		WorkflowRun: &ChainloopMetadataWorkflowRun{
			ID:         "beefdead",
			State:      "success",
			StartedAt:  date,
			FinishedAt: date.Add(10 * time.Minute),
			RunnerType: "github-actions",
			RunURL:     "chainloop.dev/runner",
		},
	}
}

type helperTestSuite struct {
	suite.Suite
	m *ChainloopMetadata
}

func testPredicate(filePath string) (chainloop.NormalizablePredicate, error) {
	var envelope dsse.Envelope
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(content, &envelope)
	if err != nil {
		return nil, err
	}

	return chainloop.ExtractPredicate(&envelope)
}
