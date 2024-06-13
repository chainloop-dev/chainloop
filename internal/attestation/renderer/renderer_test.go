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
	"encoding/json"
	"encoding/pem"
	"os"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/signer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/attestation/signer/cosign"
	"github.com/rs/zerolog"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/sigstore/pkg/signature"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
	"github.com/stretchr/testify/suite"
)

type rendererSuite struct {
	suite.Suite

	sv           signature.SignerVerifier
	dsseVerifier *dsse.EnvelopeVerifier
	cs           *v1.CraftingState
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(rendererSuite))
}

func (s *rendererSuite) SetupTest() {
	var err error
	s.cs = &v1.CraftingState{
		InputSchema: nil,
		Attestation: &v1.Attestation{
			Workflow: &v1.WorkflowMetadata{
				Name: "my-wf",
			},
		},
	}
	s.sv, _, err = signature.NewECDSASignerVerifier(elliptic.P256(), rand.Reader, crypto.SHA256)
	s.Require().NoError(err)
	s.dsseVerifier, err = dsse.NewEnvelopeVerifier(&sigdsee.VerifierAdapter{SignatureVerifier: s.sv})
	s.Require().NoError(err)
}

func (s *rendererSuite) TestRender() {
	s.Run("generated envelope is always well-formed", func() {
		renderer, err := NewAttestationRenderer(s.cs, "", "", s.sv)
		s.Require().NoError(err)

		envelope, err := renderer.Render()
		s.NoError(err)

		_, err = s.dsseVerifier.Verify(context.TODO(), envelope)
		s.NoError(err)
	})

	s.Run("simulates double wrapping bug", func() {
		doubleWrapper := sigdsee.WrapSigner(s.sv, "application/vnd.in-toto+json")

		renderer, err := NewAttestationRenderer(s.cs, "", "", doubleWrapper)
		s.Require().NoError(err)

		envelope, err := renderer.Render()
		s.NoError(err)

		_, err = s.dsseVerifier.Verify(context.TODO(), envelope)
		s.Error(err)
	})
}

func (s *rendererSuite) TestEnvelopeToBundle() {
	s.Run("from cosign signer, it doesn't generate any verification material", func() {
		envelope, err := testEnvelope("chainloop/testdata/valid.envelope.v2.json")
		s.Require().NoError(err)

		signer := cosign.NewSigner("", zerolog.Nop())
		renderer, err := NewAttestationRenderer(s.cs, "", "", signer)
		s.Require().NoError(err)

		bundle, err := renderer.envelopeToBundle(*envelope)
		s.Require().NoError(err)

		s.Equal("application/vnd.dev.sigstore.bundle+json;version=0.3", bundle.MediaType)
		s.Equal("application/vnd.in-toto+json", bundle.GetDsseEnvelope().GetPayloadType())
		s.Nil(bundle.GetVerificationMaterial().GetContent())
	})

	s.Run("from keyless signer, it adds intermediate certificates, but not the root CA", func() {
		envelope, err := testEnvelope("chainloop/testdata/valid.envelope.v2.json")
		s.Require().NoError(err)

		cert, err := testCert("chainloop/testdata/cert.pem")
		s.Require().NoError(err)

		signer := chainloop.NewSigner(nil, zerolog.Nop())
		signer.Signer = s.sv

		// 2 certs
		signer.Chain = []string{cert, "ROOT"}
		renderer, err := NewAttestationRenderer(s.cs, "", "", signer)
		s.Require().NoError(err)

		bundle, err := renderer.envelopeToBundle(*envelope)
		s.Require().NoError(err)

		s.Equal("application/vnd.dev.sigstore.bundle+json;version=0.3", bundle.MediaType)
		s.Equal("application/vnd.in-toto+json", bundle.GetDsseEnvelope().GetPayloadType())

		// only 1 cert is added
		s.Equal(1, len(bundle.GetVerificationMaterial().GetX509CertificateChain().GetCertificates()))

		// and it's the leaf certificate
		s.Equal(cert, string(pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: bundle.GetVerificationMaterial().GetX509CertificateChain().GetCertificates()[0].RawBytes}),
		))
	})
}

func testEnvelope(filePath string) (*dsse.Envelope, error) {
	var envelope dsse.Envelope
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(content, &envelope)
	if err != nil {
		return nil, err
	}

	return &envelope, nil
}

func testCert(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
