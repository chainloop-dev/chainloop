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

package biz

import (
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/ca"
	"github.com/sigstore/fulcio/pkg/identity"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

type SigningUseCase struct {
	CAs *ca.CertificateAuthorities
}

func NewChainloopSigningUseCase(cas *ca.CertificateAuthorities) *SigningUseCase {
	return &SigningUseCase{CAs: cas}
}

// CreateSigningCert signs a certificate request with a configured CA, and returns the full certificate chain
func (s *SigningUseCase) CreateSigningCert(ctx context.Context, orgID string, csrRaw []byte) ([]string, error) {
	if s.CAs == nil {
		return nil, NewErrNotImplemented("CA not initialized")
	}

	var publicKey crypto.PublicKey

	if len(csrRaw) == 0 {
		return nil, errors.New("csr cannot be empty")
	}

	// Parse CSR
	csr, err := cryptoutils.ParseCSR(csrRaw)
	if err != nil {
		return nil, fmt.Errorf("parsing csr: %w", err)
	}

	// Parse public key and check for weak key parameters
	publicKey = csr.PublicKey
	if err := cryptoutils.ValidatePubKey(publicKey); err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	// Check the CSR signature is valid
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("invalid signature: %w", err)
	}

	// Create certificate from CA provider (no Signed Certificate Timestamps for now)
	issuerCA, err := s.CAs.GetSignerCA()
	if err != nil {
		return nil, fmt.Errorf("getting signer CA: %w", err)
	}
	csc, err := issuerCA.CreateCertificateFromCSR(ctx, newChainloopPrincipal(orgID), csr)
	if err != nil {
		return nil, fmt.Errorf("creating certificate: %w", err)
	}

	// Generated certificate
	finalPEM, err := csc.CertPEM()
	if err != nil {
		return nil, fmt.Errorf("marshaling certificate to PEM: %w", err)
	}

	// Certificate chain
	finalChainPEM, err := csc.ChainPEM()
	if err != nil {
		return nil, fmt.Errorf("marshaling chain to PEM: %w", err)
	}

	return append([]string{finalPEM}, finalChainPEM...), nil
}

func (s *SigningUseCase) GetTrustedRoot(ctx context.Context) (*TrustedRoot, error) {
	trustedRoot := &TrustedRoot{Keys: make(map[string][]string)}
	for _, auth := range s.CAs.GetAuthorities() {
		chain, err := auth.GetRootChain(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting root chain: %w", err)
		}
		if len(chain) == 0 {
			continue
		}
		keyID := fmt.Sprintf("%x", sha256.Sum256(chain[0].SubjectKeyId))
		for _, cert := range chain {
			pemCert, err := cryptoutils.MarshalCertificateToPEM(cert)
			if err != nil {
				return nil, fmt.Errorf("marshaling certificate to PEM: %w", err)
			}
			trustedRoot.Keys[keyID] = append(trustedRoot.Keys[keyID], string(pemCert))
		}
	}

	return trustedRoot, nil
}

type TrustedRoot struct {
	// map of keyID and PEM encoded certificates
	Keys map[string][]string
}

type chainloopPrincipal struct {
	orgID string
}

var _ identity.Principal = (*chainloopPrincipal)(nil)

func newChainloopPrincipal(orgID string) *chainloopPrincipal {
	return &chainloopPrincipal{orgID: orgID}
}

func (p *chainloopPrincipal) Name(_ context.Context) string {
	return p.orgID
}

func (p *chainloopPrincipal) Embed(_ context.Context, cert *x509.Certificate) error {
	// no op.
	// TODO: Chainloop might have their own private enterprise number with the Internet Assigned Numbers Authority
	// 		 to embed its own identity information in the resulting certificate
	cert.Subject = pkix.Name{Organization: []string{p.orgID}}

	return nil
}
