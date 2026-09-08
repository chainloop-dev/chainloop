//
// Copyright 2024-2026 The Chainloop Authors.
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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
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

func TestPBPolicyEvaluationsRefToAction(t *testing.T) {
	const digest = "sha256:abc123"

	testCases := []struct {
		name string
		in   *pb.PolicyEvaluationsRef
		want *PolicyEvaluationsRef
	}{
		{
			name: "no reference",
		},
		{
			name: "inlined evaluations carry no reason",
			in: &pb.PolicyEvaluationsRef{
				Digest:    digest,
				SizeBytes: 2048,
				MediaType: "application/vnd.chainloop.policy-evaluations.v1+json",
				Inlined:   true,
			},
			want: &PolicyEvaluationsRef{
				Digest:    digest,
				SizeBytes: 2048,
				MediaType: "application/vnd.chainloop.policy-evaluations.v1+json",
				Inlined:   true,
			},
		},
		{
			name: "oversized bundle",
			in: &pb.PolicyEvaluationsRef{
				Digest:    digest,
				SizeBytes: 64 * 1024 * 1024,
				Reason:    pb.PolicyEvaluationsRef_REASON_TOO_LARGE,
			},
			want: &PolicyEvaluationsRef{
				Digest:    digest,
				SizeBytes: 64 * 1024 * 1024,
				Reason:    PolicyEvaluationsRefReasonTooLarge,
			},
		},
		{
			name: "unavailable bundle",
			in: &pb.PolicyEvaluationsRef{
				Digest: digest,
				Reason: pb.PolicyEvaluationsRef_REASON_UNAVAILABLE,
			},
			want: &PolicyEvaluationsRef{
				Digest: digest,
				Reason: PolicyEvaluationsRefReasonUnavailable,
			},
		},
		{
			name: "an unspecified reason without inlining stays conservative",
			in: &pb.PolicyEvaluationsRef{
				Digest: digest,
			},
			want: &PolicyEvaluationsRef{
				Digest: digest,
				Reason: PolicyEvaluationsRefReasonUnavailable,
			},
		},
		{
			name: "an unknown reason alongside inlining does not fabricate a failure",
			in: &pb.PolicyEvaluationsRef{
				Digest:  digest,
				Reason:  pb.PolicyEvaluationsRef_Reason(99),
				Inlined: true,
			},
			want: &PolicyEvaluationsRef{
				Digest:  digest,
				Inlined: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, pbPolicyEvaluationsRefToAction(tc.in))
		})
	}
}
