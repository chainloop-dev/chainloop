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

package action

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/suite"
)

type WorkflowRunDescribeTestSuite struct {
	suite.Suite
}

func TestWorkflowRunDescribe(t *testing.T) {
	suite.Run(t, new(WorkflowRunDescribeTestSuite))
}

func (s *WorkflowRunDescribeTestSuite) SetupTest() {

}

func (s *WorkflowRunDescribeTestSuite) TestVerifyEnvelope() {
	s.Run("fails if no key or cert is provided", func() {
		err := verifyEnvelope(context.TODO(), nil, &WorkflowRunDescribeOpts{})
		s.Error(err, "no public key or cert path specified")
	})

	s.Run("verifies when signed with cosign", func() {
		envelope, err := readEnvelope("testdata/cosign-attestation.json")
		s.Require().NoError(err)
		err = verifyEnvelope(context.TODO(), envelope, &WorkflowRunDescribeOpts{PublicKeyRef: "testdata/cosign.pub"})
		s.NoError(err)
	})

	s.Run("verifies when signed with certificate", func() {
		envelope, err := readEnvelope("testdata/cert-attestation.json")
		s.Require().NoError(err)
		err = verifyEnvelope(context.TODO(), envelope, &WorkflowRunDescribeOpts{
			CertPath:      "testdata/cert.pem",
			CertChainPath: "testdata/ca.pub",
		})
		s.NoError(err)
	})
}

func readEnvelope(path string) (*dsse.Envelope, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var envelope dsse.Envelope
	err = json.Unmarshal(f, &envelope)
	if err != nil {
		return nil, err
	}
	return &envelope, nil
}
