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
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/cosign/v2/pkg/signature"
	sigstoresigner "github.com/sigstore/sigstore/pkg/signature"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
	"golang.org/x/term"
	"google.golang.org/protobuf/encoding/protojson"
)

type AttestationRenderer struct {
	logger               zerolog.Logger
	signingKeyPath       string
	att                  *v1.Attestation
	renderer             r
	signingServiceClient pb.SigningServiceClient
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

func WithCPSigningClient(sc pb.SigningServiceClient) Opt {
	return func(ar *AttestationRenderer) {
		ar.signingServiceClient = sc
	}
}

func NewAttestationRenderer(state *v1.CraftingState, keyPath, builderVersion, builderDigest string, opts ...Opt) (*AttestationRenderer, error) {
	if state.GetAttestation() == nil {
		return nil, errors.New("attestation not initialized")
	}

	r := &AttestationRenderer{
		logger:         zerolog.Nop(),
		signingKeyPath: keyPath,
		att:            state.GetAttestation(),
		renderer:       chainloop.NewChainloopRendererV02(state.GetAttestation(), builderVersion, builderDigest),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r, nil
}

// Attestation (dsee envelope) -> { message: { Statement(in-toto): [subject, predicate] }, signature: "sig" }.
// NOTE: It currently only supports cosign key based signing.
func (ab *AttestationRenderer) Render(ctx context.Context) (*dsse.Envelope, error) {
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

	wrappedSigner, err := ab.getSigner(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting signer: %w", err)
	}

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

func (ab *AttestationRenderer) getSigner(ctx context.Context) (sigstoresigner.Signer, error) {
	var (
		signer sigstoresigner.Signer
		err    error
	)

	if ab.signingKeyPath != "" {
		ab.logger.Debug().Str("path", ab.signingKeyPath).Msg("loading key")
		signer, err = signature.SignerFromKeyRef(context.Background(), ab.signingKeyPath, getPass)
		if err != nil {
			return nil, err
		}
	} else {
		// key is not provided, let's create one
		ab.logger.Info().Msg("key not provided, running in key-less mode")
		signer, err = ab.keyLessSigner(ctx)
		if err != nil {
			return nil, err
		}
	}

	return sigdsee.WrapSigner(signer, "application/vnd.in-toto+json"), nil
}

type certificateRequest struct {
	PrivateKey *ecdsa.PrivateKey
	// CertificateRequestPEM contains the signed public key and the CSR metadata
	CertificateRequestPEM []byte
}

type Signer struct {
	sigstoresigner.SignerVerifier
	Cert  string
	Chain []string
}

func (ab *AttestationRenderer) keyLessSigner(ctx context.Context) (sigstoresigner.Signer, error) {
	request, err := ab.createCertificateRequest()
	if err != nil {
		return nil, err
	}
	certs, err := ab.certFromChainloop(ctx, request)
	if err != nil {
		return nil, err
	}
	sv, err := sigstoresigner.LoadECDSASignerVerifier(request.PrivateKey, crypto.SHA256)
	if err != nil {
		return nil, err
	}

	var chain []string
	if len(certs) > 1 {
		chain = append(chain, certs[1:]...)
	}

	return &Signer{
		Cert:           certs[0],
		Chain:          chain,
		SignerVerifier: sv,
	}, nil
}

func (ab *AttestationRenderer) createCertificateRequest() (*certificateRequest, error) {
	ab.logger.Debug().Msg("generating new certificate request")

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating cert: %w", err)
	}
	csrTmpl := &x509.CertificateRequest{Subject: pkix.Name{CommonName: "ephemeral certificate"}}
	derCSR, err := x509.CreateCertificateRequest(rand.Reader, csrTmpl, priv)
	if err != nil {
		return nil, fmt.Errorf("generating certificate request: %w", err)
	}

	// Encode CSR to PEM
	pemCSR := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: derCSR,
	})

	return &certificateRequest{
		CertificateRequestPEM: pemCSR,
		PrivateKey:            priv,
	}, nil
}

func (ab *AttestationRenderer) certFromChainloop(ctx context.Context, req *certificateRequest) ([]string, error) {
	cr := pb.SigningCertRequest{
		CertificateSigningRequest: req.CertificateRequestPEM,
	}

	// call chainloop
	resp, err := ab.signingServiceClient.SigningCert(ctx, &cr)
	if err != nil {
		return nil, err
	}

	// get full chain
	return resp.GetChain().GetCertificates(), nil
}
