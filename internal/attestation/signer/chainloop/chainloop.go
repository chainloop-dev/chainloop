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

package chainloop

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"sync"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/rs/zerolog"
	sigstoresigner "github.com/sigstore/sigstore/pkg/signature"
)

// Signer is a keyless signer for Chainloop
type Signer struct {
	sigstoresigner.Signer

	// PEM encoded public certificate chain
	Chain []string

	// where to write the certificate chain to
	signingServiceClient pb.SigningServiceClient
	logger               zerolog.Logger
	mu                   sync.Mutex
}

var _ sigstoresigner.Signer = (*Signer)(nil)

func NewSigner(sc pb.SigningServiceClient, logger zerolog.Logger) *Signer {
	return &Signer{
		signingServiceClient: sc,
		logger:               logger,
	}
}

func (cs *Signer) SignMessage(message io.Reader, opts ...sigstoresigner.SignOption) ([]byte, error) {
	err := cs.ensureInitiated(context.Background())
	if err != nil {
		return nil, err
	}

	return cs.Signer.SignMessage(message, opts...)
}

// ensureInitiated makes sure the signer is fully initialized and can be used right away
// (i.e. it has performed the CSR challenge with chainloop)
func (cs *Signer) ensureInitiated(ctx context.Context) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.Signer != nil {
		return nil
	}

	var err error

	// key is not provided, let's create one
	cs.logger.Debug().Msg("generating a keyless signer")
	cs.Signer, err = cs.keyLessSigner(ctx)
	if err != nil {
		return fmt.Errorf("getting a keyless signer: %w", err)
	}

	return nil
}

type certificateRequest struct {
	PrivateKey *ecdsa.PrivateKey
	// CertificateRequestPEM contains the signed public key and the CSR metadata
	CertificateRequestPEM []byte
}

func (cs *Signer) keyLessSigner(ctx context.Context) (sigstoresigner.Signer, error) {
	request, err := cs.createCertificateRequest()
	if err != nil {
		return nil, fmt.Errorf("creating certificate request: %w", err)
	}
	cs.Chain, err = cs.certFromChainloop(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("getting a certificate from chainloop: %w", err)
	}
	sv, err := sigstoresigner.LoadECDSASignerVerifier(request.PrivateKey, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("loading ECDSA signer from private key: %w", err)
	}

	return sv, nil
}

// createCertificateRequest generates a new CSR to be sent to Chainloop platform
func (cs *Signer) createCertificateRequest() (*certificateRequest, error) {
	cs.logger.Debug().Msg("generating new certificate request")

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

// certFromChainloop gets a full certificate chain from a CSR
func (cs *Signer) certFromChainloop(ctx context.Context, req *certificateRequest) ([]string, error) {
	cr := pb.GenerateSigningCertRequest{
		CertificateSigningRequest: req.CertificateRequestPEM,
	}

	// call chainloop
	resp, err := cs.signingServiceClient.GenerateSigningCert(ctx, &cr)
	if err != nil {
		return nil, fmt.Errorf("generating signing cert: %w", err)
	}

	// get full chain
	return resp.GetChain().GetCertificates(), nil
}
