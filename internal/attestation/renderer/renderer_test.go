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

package renderer

import (
	"context"
	"crypto"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/sigstore/pkg/signature"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
	"github.com/stretchr/testify/suite"
)

type rendererSuite struct {
	suite.Suite
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(rendererSuite))
}

func (s *rendererSuite) TestRender() {
	sv, _, err := signature.NewECDSASignerVerifier(elliptic.P256(), rand.Reader, crypto.SHA256)
	s.Require().NoError(err)
	ws := sigdsee.WrapSigner(sv, "application/vnd.in-toto+json")

	cs := &v1.CraftingState{
		InputSchema: nil,
		Attestation: &v1.Attestation{
			Workflow: &v1.WorkflowMetadata{
				Name: "my-wf",
			},
		},
	}
	renderer, err := NewAttestationRenderer(cs, "", "", ws)
	s.Require().NoError(err)

	envelope, err := renderer.Render()
	s.NoError(err)

	dsseVerifier, err := dsse.NewEnvelopeVerifier(&sigdsee.VerifierAdapter{SignatureVerifier: sv})
	s.NoError(err)

	_, err = dsseVerifier.Verify(context.TODO(), envelope)
	s.NoError(err)
}
