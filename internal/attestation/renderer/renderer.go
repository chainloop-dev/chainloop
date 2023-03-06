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
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	v1 "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/rs/zerolog"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/cosign/pkg/signature"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
	"golang.org/x/term"
)

type AttestationRenderer struct {
	logger         *zerolog.Logger
	signingKeyPath string
	att            *v1.Attestation
	renderer       r
}

type r interface {
	Header() (*in_toto.StatementHeader, error)
	Predicate() (interface{}, error)
}

func NewAttestationRenderer(state *v1.CraftingState, keyPath, builderVersion, builderDigest string, logger *zerolog.Logger) (*AttestationRenderer, error) {
	if state.GetAttestation() == nil {
		return nil, errors.New("attestation not initialized")
	}

	return &AttestationRenderer{
		logger:         logger,
		signingKeyPath: keyPath,
		att:            state.GetAttestation(),
		renderer:       newChainloopRenderer(state.GetAttestation(), state.GetInputSchema(), builderVersion, builderDigest),
	}, nil
}

// Attestation (dsee envelope) -> { message: { Statement(in-toto): [subject, predicate] }, signature: "sig" }.
// NOTE: It currently only supports cosign key based signing.
func (ab *AttestationRenderer) Render() (*dsse.Envelope, error) {
	ab.logger.Debug().Msg("generating in-toto statement")

	statement, err := ab.statement()
	if err != nil {
		return nil, err
	}

	rawStatement, err := json.Marshal(statement)
	if err != nil {
		return nil, err
	}

	ab.logger.Debug().Str("path", ab.signingKeyPath).Msg("loading key")

	signer, err := signature.SignerFromKeyRef(context.Background(), ab.signingKeyPath, getPass)
	if err != nil {
		return nil, err
	}

	wrappedSigner := sigdsee.WrapSigner(signer, in_toto.PayloadType)

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

func (ab *AttestationRenderer) statement() (*in_toto.Statement, error) {
	header, err := ab.renderer.Header()
	if err != nil {
		return nil, err
	}

	predicate, err := ab.renderer.Predicate()
	if err != nil {
		return nil, err
	}

	return &in_toto.Statement{
		StatementHeader: *header,
		Predicate:       predicate,
	}, nil
}

func getPass(confirm bool) ([]byte, error) {
	read := readPasswordFn(confirm)
	return read()
}

// based on cosign code
// https://pkg.go.dev/github.com/sigstore/cosign/cmd/cosign/cli/generate
func readPasswordFn(confirm bool) func() ([]byte, error) {
	pw, ok := os.LookupEnv("CHAINLOOP_SIGNING_PASSWORD")
	switch {
	case ok:
		return func() ([]byte, error) {
			return []byte(pw), nil
		}
	case isTerminal():
		return func() ([]byte, error) {
			return getPassFromTerm(confirm)
		}
	// Handle piped in passwords.
	default:
		return func() ([]byte, error) {
			return io.ReadAll(os.Stdin)
		}
	}
}

func isTerminal() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func getPassFromTerm(confirm bool) ([]byte, error) {
	fmt.Fprint(os.Stderr, "Enter password for private key: ")
	// Unnecessary convert of syscall.Stdin on *nix, but Windows is a uintptr
	// nolint:unconvert
	pw1, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(os.Stderr)
	if !confirm {
		return pw1, nil
	}
	fmt.Fprint(os.Stderr, "Enter password for private key again: ")
	// Unnecessary convert of syscall.Stdin on *nix, but Windows is a uintptr
	// nolint:unconvert
	confirmpw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}

	if string(pw1) != string(confirmpw) {
		return nil, errors.New("passwords do not match")
	}
	return pw1, nil
}
