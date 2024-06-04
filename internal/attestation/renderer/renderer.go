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
	"encoding/json"
	"errors"
	"fmt"

	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	sigstoresigner "github.com/sigstore/sigstore/pkg/signature"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
	"google.golang.org/protobuf/encoding/protojson"
)

type AttestationRenderer struct {
	logger         zerolog.Logger
	signingKeyPath string
	att            *v1.Attestation
	renderer       r
	signer         sigstoresigner.Signer
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

func NewAttestationRenderer(state *v1.CraftingState, keyPath, builderVersion, builderDigest string, signer sigstoresigner.Signer, opts ...Opt) (*AttestationRenderer, error) {
	if state.GetAttestation() == nil {
		return nil, errors.New("attestation not initialized")
	}

	r := &AttestationRenderer{
		logger:         zerolog.Nop(),
		signingKeyPath: keyPath,
		att:            state.GetAttestation(),
		signer:         signer,
		renderer:       chainloop.NewChainloopRendererV02(state.GetAttestation(), builderVersion, builderDigest),
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

	wrappedSigner := sigdsee.WrapSigner(ab.signer, "application/vnd.in-toto+json")
	signedAtt, err := wrappedSigner.SignMessage(bytes.NewReader(rawStatement))
	if err != nil {
		return nil, fmt.Errorf("signing message: %w", err)
	}

	var dseeEnvelope dsse.Envelope
	if err := json.Unmarshal(signedAtt, &dseeEnvelope); err != nil {
		return nil, err
	}

	return &dseeEnvelope, nil
}
