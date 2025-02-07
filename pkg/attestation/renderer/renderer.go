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
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	chainloopsigner "github.com/chainloop-dev/chainloop/pkg/attestation/signer/chainloop"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	v12 "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	sigstoresigner "github.com/sigstore/sigstore/pkg/signature"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
	"google.golang.org/protobuf/encoding/protojson"
)

type AttestationRenderer struct {
	logger     zerolog.Logger
	att        *v1.Attestation
	schema     *schemaapi.CraftingSchema
	renderer   r
	signer     sigstoresigner.Signer
	dsseSigner sigstoresigner.Signer
	bundlePath string
	attClient  pb.AttestationServiceClient
}

type r interface {
	Statement(ctx context.Context) (*intoto.Statement, error)
}

type Opt func(*AttestationRenderer)

func WithLogger(logger zerolog.Logger) Opt {
	return func(ar *AttestationRenderer) {
		ar.logger = logger
	}
}

func WithBundleOutputPath(bundlePath string) Opt {
	return func(ar *AttestationRenderer) {
		ar.bundlePath = bundlePath
	}
}

func NewAttestationRenderer(state *crafter.VersionedCraftingState, attClient pb.AttestationServiceClient, builderVersion, builderDigest string, signer sigstoresigner.Signer, opts ...Opt) (*AttestationRenderer, error) {
	if state.GetAttestation() == nil {
		return nil, errors.New("attestation not initialized")
	}

	r := &AttestationRenderer{
		logger:     zerolog.Nop(),
		att:        state.GetAttestation(),
		schema:     state.GetInputSchema(),
		dsseSigner: sigdsee.WrapSigner(signer, "application/vnd.in-toto+json"),
		signer:     signer,
		attClient:  attClient,
	}

	for _, opt := range opts {
		opt(r)
	}

	r.renderer = chainloop.NewChainloopRendererV02(state.GetAttestation(), state.GetInputSchema(), builderVersion, builderDigest, attClient, &r.logger)

	return r, nil
}

// Render the in-toto statement skipping validations, dsse envelope wrapping nor signing
func (ab *AttestationRenderer) RenderStatement(ctx context.Context) (*intoto.Statement, error) {
	statement, err := ab.renderer.Statement(ctx)
	if err != nil {
		return nil, fmt.Errorf("generating in-toto statement: %w", err)
	}

	return statement, nil
}

// Attestation (dsee envelope) -> { message: { Statement(in-toto): [subject, predicate] }, signature: "sig" }.
// NOTE: It currently only supports cosign key based signing.
func (ab *AttestationRenderer) Render(ctx context.Context) (*dsse.Envelope, *protobundle.Bundle, error) {
	ab.logger.Debug().Msg("generating in-toto statement")

	statement, err := ab.renderer.Statement(ctx)
	if err != nil {
		return nil, nil, err
	}

	if err := statement.Validate(); err != nil {
		return nil, nil, fmt.Errorf("validating intoto statement: %w", err)
	}

	rawStatement, err := protojson.Marshal(statement)
	if err != nil {
		return nil, nil, err
	}

	signedAtt, err := ab.dsseSigner.SignMessage(bytes.NewReader(rawStatement))
	if err != nil {
		return nil, nil, fmt.Errorf("signing message: %w", err)
	}

	var dsseEnvelope dsse.Envelope
	if err := json.Unmarshal(signedAtt, &dsseEnvelope); err != nil {
		return nil, nil, err
	}

	// Create sigstore bundle for the contents of this attestation
	bundle, err := ab.envelopeToBundle(&dsseEnvelope)
	if err != nil {
		return nil, nil, fmt.Errorf("loading bundle: %w", err)
	}
	json, err := protojson.Marshal(bundle)
	if err != nil {
		return nil, nil, fmt.Errorf("marshalling bundle: %w", err)
	}

	if ab.bundlePath != "" {
		ab.logger.Info().Msg(fmt.Sprintf("Storing Sigstore bundle %s", ab.bundlePath))
		err = os.WriteFile(ab.bundlePath, json, 0600)
		if err != nil {
			return nil, nil, fmt.Errorf("writing bundle: %w", err)
		}
	}

	return &dsseEnvelope, bundle, nil
}

func (ab *AttestationRenderer) envelopeToBundle(dsseEnvelope *dsse.Envelope) (*protobundle.Bundle, error) {
	bundle, err := attestation.BundleFromDSSEEnvelope(dsseEnvelope)
	if err != nil {
		return nil, err
	}

	// extract verification materials
	// Note: we don't support PublicKey materials (from cosign.key and KMS signers), since Chainloop doesn't (yet) store
	//       public keys.
	if v, ok := ab.signer.(*chainloopsigner.Signer); ok {
		chain := v.Chain
		if len(chain) == 0 {
			return nil, errors.New("certificate chain is empty")
		}

		// Store generated cert, ignoring the chain
		block, _ := pem.Decode([]byte(chain[0]))
		if block == nil {
			return nil, fmt.Errorf("failed to decode PEM block")
		}

		cert := &v12.X509Certificate{RawBytes: block.Bytes}
		bundle.VerificationMaterial.Content = &protobundle.VerificationMaterial_Certificate{
			Certificate: cert,
		}
	}

	return bundle, nil
}
