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

package renderer

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	chainloopsigner "github.com/chainloop-dev/chainloop/internal/attestation/signer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/attestation/signer/cosign"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	v12 "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	dsse2 "github.com/sigstore/protobuf-specs/gen/pb-go/dsse"
	sigstoresigner "github.com/sigstore/sigstore/pkg/signature"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
	"google.golang.org/protobuf/encoding/protojson"
)

type AttestationRenderer struct {
	logger     zerolog.Logger
	att        *v1.Attestation
	renderer   r
	signer     sigstoresigner.Signer
	dsseSigner sigstoresigner.Signer
	bundlePath string
}

type r interface {
	Statement() (*intoto.Statement, error)
}

type Opt func(*AttestationRenderer)

func WithLogger(logger zerolog.Logger) Opt {
	return func(ar *AttestationRenderer) {
		ar.logger = logger
	}
}

func WithBundle(bundlePath string) Opt {
	return func(ar *AttestationRenderer) {
		ar.bundlePath = bundlePath
	}
}

func NewAttestationRenderer(state *v1.CraftingState, builderVersion, builderDigest string, signer sigstoresigner.Signer, opts ...Opt) (*AttestationRenderer, error) {
	if state.GetAttestation() == nil {
		return nil, errors.New("attestation not initialized")
	}

	r := &AttestationRenderer{
		logger:     zerolog.Nop(),
		att:        state.GetAttestation(),
		dsseSigner: sigdsee.WrapSigner(signer, "application/vnd.in-toto+json"),
		signer:     signer,
		renderer:   chainloop.NewChainloopRendererV02(state.GetAttestation(), builderVersion, builderDigest),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r, nil
}

// Attestation (dsee envelope) -> { message: { Statement(in-toto): [subject, predicate] }, signature: "sig" }.
// NOTE: It currently only supports cosign key based signing.
func (ab *AttestationRenderer) Render() (*dsse.Envelope, error) {
	ab.logger.Debug().Msg("generating in-toto statement")

	statement, err := ab.renderer.Statement()
	if err != nil {
		return nil, err
	}

	if err := statement.Validate(); err != nil {
		return nil, fmt.Errorf("validating intoto statement: %w", err)
	}

	rawStatement, err := protojson.Marshal(statement)
	if err != nil {
		return nil, err
	}

	signedAtt, err := ab.dsseSigner.SignMessage(bytes.NewReader(rawStatement))
	if err != nil {
		return nil, fmt.Errorf("signing message: %w", err)
	}

	var dsseEnvelope dsse.Envelope
	if err := json.Unmarshal(signedAtt, &dsseEnvelope); err != nil {
		return nil, err
	}

	if ab.bundlePath != "" {
		// Create sigstore bundle for the contents of this attestation
		bundle, err := ab.envelopeToBundle(dsseEnvelope)
		if err != nil {
			return nil, fmt.Errorf("loading bundle: %w", err)
		}
		json, err := protojson.Marshal(bundle)
		if err != nil {
			return nil, fmt.Errorf("marshalling bundle: %w", err)
		}
		err = os.WriteFile(ab.bundlePath, json, 0600)
		if err != nil {
			return nil, fmt.Errorf("writing bundle: %w", err)
		}
	}

	return &dsseEnvelope, nil
}

func (ab *AttestationRenderer) envelopeToBundle(dsseEnvelope dsse.Envelope) (*protobundle.Bundle, error) {
	// DSSE Envelope is already base64 encoded, we need to decode to prevent it from being encoded twice
	payload, err := base64.StdEncoding.DecodeString(dsseEnvelope.Payload)
	if err != nil {
		return nil, fmt.Errorf("decoding: %w", err)
	}
	bundle := &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle+json;version=0.3",
		Content: &protobundle.Bundle_DsseEnvelope{DsseEnvelope: &dsse2.Envelope{
			Payload:     payload,
			PayloadType: dsseEnvelope.PayloadType,
			Signatures: []*dsse2.Signature{
				{
					Sig:   []byte(dsseEnvelope.Signatures[0].Sig),
					Keyid: dsseEnvelope.Signatures[0].KeyID,
				},
			},
		}},
		VerificationMaterial: &protobundle.VerificationMaterial{},
	}

	// check type of wrapped signer
	switch v := ab.signer.(type) {
	case *cosign.Signer:
		pk, err := v.PublicKey()
		if err != nil {
			return nil, fmt.Errorf("getting public key: %w", err)
		}
		pkContent, err := x509.MarshalPKIXPublicKey(pk)
		if err != nil {
			return nil, fmt.Errorf("marshalling public key: %w", err)
		}
		bundle.VerificationMaterial.Content = &protobundle.VerificationMaterial_PublicKey{PublicKey: &v12.PublicKeyIdentifier{
			Hint: string(pkContent),
		}}
		break
	case *chainloopsigner.Signer:
		chain := v.Chain
		certs := make([]*v12.X509Certificate, 0)
		// Store cert chain except root certificate, as it's required to be provided separately
		for _, c := range chain[0 : len(chain)-1] {
			block, _ := pem.Decode([]byte(c))
			if block == nil {
				return nil, fmt.Errorf("failed to decode PEM block")
			}
			certs = append(certs, &v12.X509Certificate{RawBytes: block.Bytes})
		}
		bundle.VerificationMaterial.Content = &protobundle.VerificationMaterial_X509CertificateChain{
			X509CertificateChain: &v12.X509CertificateChain{
				Certificates: certs,
			},
		}
	}

	return bundle, nil
}
