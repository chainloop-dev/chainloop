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
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"

	"github.com/sigstore/fulcio/pkg/ca"
	"github.com/sigstore/fulcio/pkg/identity"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

type SigningCertCreator interface {
	// CreateSigningCert creates a signing certificate from and returns the certificate chain
	CreateSigningCert(context.Context, string, []byte) ([]string, error)
}

type SigningUseCase struct {
	CA ca.CertificateAuthority
}

var _ SigningCertCreator = (*SigningUseCase)(nil)

func NewChainloopSigningUseCase(ca ca.CertificateAuthority) *SigningUseCase {
	return &SigningUseCase{CA: ca}
}

func (s *SigningUseCase) CreateSigningCert(ctx context.Context, orgId string, csrRaw []byte) ([]string, error) {
	var publicKey crypto.PublicKey

	if len(csrRaw) == 0 {
		return nil, errors.New("csr cannot be empty")
	}

	// Parse CSR
	csr, err := cryptoutils.ParseCSR(csrRaw)
	if err != nil {
		return nil, err
	}

	// Parse public key and check for weak key parameters
	publicKey = csr.PublicKey
	if err := cryptoutils.ValidatePubKey(publicKey); err != nil {
		return nil, err
	}

	// Check the CSR signature is valid
	if err := csr.CheckSignature(); err != nil {
		return nil, err
	}

	// Create certificate from CA provider (no Signed Certificate Timestamps for now)
	csc, err := s.CA.CreateCertificate(ctx, NewChainloopPrincipal(orgId), publicKey)
	if err != nil {
		return nil, err
	}

	// Generated certificate
	finalPEM, err := csc.CertPEM()
	if err != nil {
		return nil, err
	}

	// Certificate chain
	finalChainPEM, err := csc.ChainPEM()
	if err != nil {
		return nil, err
	}

	return append([]string{finalPEM}, finalChainPEM...), nil
}

type ChainloopPrincipal struct {
	orgId string
}

var _ identity.Principal = (*ChainloopPrincipal)(nil)

func NewChainloopPrincipal(orgId string) *ChainloopPrincipal {
	return &ChainloopPrincipal{orgId: orgId}
}

func (p *ChainloopPrincipal) Name(_ context.Context) string {
	return p.orgId
}

func (p *ChainloopPrincipal) Embed(_ context.Context, cert *x509.Certificate) error {
	// no op.
	// TODO: Chainloop might have their own private enterprise number with the Internet Assigned Numbers Authority
	// 		 to embed its own identity information in the resulting certificate
	cert.Subject = pkix.Name{Organization: []string{p.orgId}}

	return nil
}
