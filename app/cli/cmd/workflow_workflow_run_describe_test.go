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

package cmd

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/suite"
)

type workflowRunDescribeSuite struct {
	suite.Suite

	run *action.WorkflowRunItemFull
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(workflowRunDescribeSuite))
}

func (s *workflowRunDescribeSuite) SetupTest() {
	s.run = &action.WorkflowRunItemFull{
		Attestation: &action.WorkflowRunAttestationItem{
			Envelope: &dsse.Envelope{
				PayloadType: "application/vnd.in-toto+json",
				Payload:     base64.StdEncoding.EncodeToString([]byte("hello")),
				Signatures:  nil,
			},
		},
	}

}

func (s *workflowRunDescribeSuite) TestOutputTypePayload() {
	flagOutputFormat = formatPayloadPAE
	expected := "DSSEv1 28 application/vnd.in-toto+json 5 hello"

	buf := new(bytes.Buffer)
	err := encodeAttestationOutput(s.run, buf)
	s.NoError(err)

	s.Require().NoError(err)
	s.Equal(expected, buf.String())
}
