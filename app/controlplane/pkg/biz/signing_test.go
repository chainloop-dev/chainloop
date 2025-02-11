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

package biz_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	ca2 "github.com/chainloop-dev/chainloop/app/controlplane/pkg/ca"
	fulcioca "github.com/sigstore/fulcio/pkg/ca"
	"github.com/sigstore/fulcio/pkg/ca/ephemeralca"
	"github.com/sigstore/fulcio/pkg/identity"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/stretchr/testify/suite"
)

type signingUseCaseTestSuite struct {
	suite.Suite
	uc  *biz.SigningUseCase
	csr []byte
}

type TestCA struct {
	ca *ephemeralca.EphemeralCA
}

func (e TestCA) CreateCertificateFromCSR(ctx context.Context, principal identity.Principal, csr *x509.CertificateRequest) (*fulcioca.CodeSigningCertificate, error) {
	return e.ca.CreateCertificate(ctx, principal, csr.PublicKey)
}

func (e TestCA) GetRootChain(ctx context.Context) ([]*x509.Certificate, error) {
	tb, err := e.ca.TrustBundle(ctx)
	if err != nil {
		return nil, err
	}
	return tb[0], nil
}

func NewTestCA() (*TestCA, error) {
	ca, err := ephemeralca.NewEphemeralCA()
	if err != nil {
		return nil, err
	}
	return &TestCA{ca}, nil
}

func (s *signingUseCaseTestSuite) TestSigningUseCase_CreateSigningCert() {
	s.Run("with empty certificate", func() {
		_, err := s.uc.CreateSigningCert(context.TODO(), "myorgid", make([]byte, 0))
		s.Error(err)
	})

	s.Run("with certificate request", func() {
		certChain, err := s.uc.CreateSigningCert(context.TODO(), "myorgid", s.csr)
		s.NoError(err)

		// assert 2 certificates: signing certificate + chain (only one)
		s.Len(certChain, 2)

		// check cert contents
		cert, err := cryptoutils.UnmarshalCertificatesFromPEM([]byte(certChain[0]))
		s.NoError(err)
		s.Len(cert, 1)
		s.Equal("myorgid", cert[0].Subject.Organization[0])
	})
}

func (s *signingUseCaseTestSuite) TestSigningUseCase_GetRootChain() {
	s.Run("gets the chain", func() {
		r, err := s.uc.GetTrustedRoot(context.TODO())
		s.NoError(err)
		s.Len(r.Keys, 1)
	})

	s.Run("it's a valid certificate", func() {
		r, err := s.uc.GetTrustedRoot(context.TODO())
		s.NoError(err)
		for k, v := range r.Keys {
			s.NotEmpty(k)
			s.Len(v, 1)
			s.Contains(v[0], "-----BEGIN CERTIFICATE-----")
		}
	})
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(signingUseCaseTestSuite))
}

func (s *signingUseCaseTestSuite) SetupTest() {
	csr, err := createCSR()
	s.Require().NoError(err)
	s.csr = csr

	ca, err := NewTestCA()
	s.Require().NoError(err)
	s.uc = &biz.SigningUseCase{CAs: &ca2.CertificateAuthorities{CAs: []ca2.CertificateAuthority{ca}}}
}

func createCSR() ([]byte, error) {
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

	return pemCSR, nil
}
