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
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"net/url"
	"os"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/ca"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/sigstore/fulcio/pkg/identity"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

type SigningUseCase struct {
	logger               *log.Helper
	CAs                  *ca.CertificateAuthorities
	TimestampAuthorities []*TimestampAuthority
}

type TimestampAuthority struct {
	Issuer    bool
	URL       *url.URL
	CertChain []*x509.Certificate
}

func NewChainloopSigningUseCase(config *conf.Bootstrap, l log.Logger) (*SigningUseCase, error) {
	logger := servicelogger.ScopedHelper(l, "biz/signing")

	tsas, err := parseTimestamps(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamps authorities: %w", err)
	}

	cas, err := parseCAs(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA authorities: %w", err)
	}

	return &SigningUseCase{CAs: cas, TimestampAuthorities: tsas, logger: logger}, nil
}

func parseTimestamps(config *conf.Bootstrap, logger *log.Helper) ([]*TimestampAuthority, error) {
	var issuerFound bool
	auths := make([]*TimestampAuthority, 0)
	for _, tsaConf := range config.GetTimestampAuthorities() {
		tsa, err := parseTSA(tsaConf)
		if err != nil {
			return nil, err
		}
		if issuerFound && tsa.Issuer {
			return nil, fmt.Errorf("duplicate timestamp issuer in tsa config")
		}
		issuerFound = tsa.Issuer
		auths = append(auths, tsa)
	}
	// set default if there's only one
	if len(auths) == 1 && auths[0].URL != nil {
		auths[0].Issuer = true
		issuerFound = true
	}
	// error if no issuer found
	if len(auths) > 0 && !issuerFound {
		return nil, fmt.Errorf("timestamp issuer not found in tsa config")
	}
	logger.Infof("timestamp authority configured with %d TSA servers", len(auths))
	return auths, nil
}

func parseCAs(config *conf.Bootstrap, logger *log.Helper) (*ca.CertificateAuthorities, error) {
	authorities, err := ca.NewCertificateAuthoritiesFromConfig(config.GetCertificateAuthorities(), logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA authorities: %w", err)
	}
	// No CA configured, keyless will be deactivated.
	if len(authorities.GetAuthorities()) == 0 {
		logger.Info("Keyless Signing NOT configured")
		return nil, nil
	}
	return authorities, nil
}

func parseTSA(tsaConf *conf.TSA) (*TimestampAuthority, error) {
	tsa := &TimestampAuthority{}
	var err error

	// we'll only require URL if it's the main one, as others will be used for verification only
	if tsaConf.Url != "" {
		tsa.URL, err = url.Parse(tsaConf.Url)
		if err != nil && tsaConf.Issuer {
			return nil, fmt.Errorf("failed to parse TSA URL: %w", err)
		}
	}

	tsa.Issuer = tsaConf.Issuer
	if tsaConf.GetCertChainPath() == "" {
		return nil, fmt.Errorf("missing certificate path for TSA")
	}
	pemBytes, err := os.ReadFile(tsaConf.GetCertChainPath())
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate chain: %w", err)
	}
	certs, err := cryptoutils.LoadCertificatesFromPEM(bytes.NewReader(pemBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to load certificates: %w", err)
	}
	tsa.CertChain = certs

	return tsa, nil
}

func (s *SigningUseCase) GetCurrentTSA() *TimestampAuthority {
	for _, tsa := range s.TimestampAuthorities {
		if tsa.Issuer {
			return tsa
		}
	}
	// Nil means not configured and needs to be handled correctly
	return nil
}

// GetSigningCA returns the current CA authority (if any) used for signing
func (s *SigningUseCase) GetSigningCA() ca.CertificateAuthority {
	// No CA configured
	if s.CAs == nil {
		return nil
	}

	// Return signing CA (it can be nil if not configured)
	return s.CAs.SignerCA
}

// CreateSigningCert signs a certificate request with a configured CA, and returns the full certificate chain
func (s *SigningUseCase) CreateSigningCert(ctx context.Context, orgID string, csrRaw []byte) ([]string, error) {
	if s.CAs == nil {
		return nil, NewErrNotImplemented("CAs not initialized")
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
	if s.CAs == nil {
		return nil, NewErrNotImplemented("CAs not initialized")
	}
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
	if len(s.TimestampAuthorities) > 0 {
		trustedRoot.TimestampAuthorities = make(map[string][]string)
		for _, authority := range s.TimestampAuthorities {
			if len(authority.CertChain) == 0 {
				continue
			}
			authorityKeyID := fmt.Sprintf("%x", sha256.Sum256(authority.CertChain[0].SubjectKeyId))
			for _, cert := range authority.CertChain {
				pemCert, err := cryptoutils.MarshalCertificateToPEM(cert)
				if err != nil {
					return nil, fmt.Errorf("marshaling certificate to PEM: %w", err)
				}
				trustedRoot.TimestampAuthorities[authorityKeyID] = append(trustedRoot.TimestampAuthorities[authorityKeyID], string(pemCert))
			}
		}
	}

	return trustedRoot, nil
}

type TrustedRoot struct {
	// map of keyID and PEM encoded certificates
	Keys map[string][]string
	// Timestamp Authorities
	TimestampAuthorities map[string][]string
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
