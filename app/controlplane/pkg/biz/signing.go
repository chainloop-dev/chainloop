//
// Copyright 2024-2026 The Chainloop Authors.
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
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/ca"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/attestation/sandboxidentity"
	"github.com/chainloop-dev/chainloop/pkg/attestation/verifier"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/sigstore/fulcio/pkg/identity"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

var signingTracer = otelx.Tracer("chainloop-controlplane", "biz/signing")

type SigningUseCase struct {
	logger               *log.Helper
	CAs                  *ca.CertificateAuthorities
	TimestampAuthorities []*TimestampAuthority
	// ForceVerification requires every attestation to be verified with keyless
	// signing, when keyless signing is configured
	ForceVerification bool
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

	// Forced verification is opt-out, so it is enabled unless it is explicitly disabled
	forceVerification := true
	if a := config.GetAttestations(); a != nil && a.ForceVerification != nil {
		forceVerification = *a.ForceVerification
	}

	if cas != nil && !forceVerification {
		logger.Warn("keyless signing verification is not forced, attestations signed with other methods are accepted")
	}

	return &SigningUseCase{CAs: cas, TimestampAuthorities: tsas, logger: logger, ForceVerification: forceVerification}, nil
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
		issuerFound = issuerFound || tsa.Issuer
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

// KeylessEnabled tells if keyless signing is configured, in other words, if
// there are certificate authorities to issue signing certificates.
func (s *SigningUseCase) KeylessEnabled() bool {
	return s != nil && s.CAs != nil
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

// SigningCertOption customizes a signing certificate.
type SigningCertOption func(*signingCertOpts)

type signingCertOpts struct {
	sandboxIdentityIssuer bool
}

// WithSandboxIdentityMark marks the certificate as able to sign the identity of a Chainloop
// sandbox. The caller must hold authz.PolicySandboxIdentityIssue.
func WithSandboxIdentityMark() SigningCertOption {
	return func(o *signingCertOpts) {
		o.sandboxIdentityIssuer = true
	}
}

// CreateSigningCert signs a certificate request with a configured CA, and returns the full certificate chain
func (s *SigningUseCase) CreateSigningCert(ctx context.Context, orgID string, csrRaw []byte, opts ...SigningCertOption) ([]string, error) {
	ctx, span := otelx.Start(ctx, signingTracer, "SigningUseCase.CreateSigningCert")
	defer span.End()

	if s.CAs == nil {
		return nil, NewErrNotImplemented("CAs not initialized")
	}

	options := &signingCertOpts{}
	for _, opt := range opts {
		opt(options)
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
	csc, err := issuerCA.CreateCertificateFromCSR(ctx, newChainloopPrincipal(orgID, options.sandboxIdentityIssuer), csr)
	if err != nil {
		return nil, fmt.Errorf("creating certificate: %w", err)
	}

	// A CA that builds the subject itself, for example from the request, does not use the
	// principal. Check the result, so that only a caller with the permission gets the mark.
	if marked := sandboxidentity.HasMark(csc.FinalCertificate); marked != options.sandboxIdentityIssuer {
		if options.sandboxIdentityIssuer {
			return nil, NewErrNotImplemented("the signing CA cannot issue certificates for a sandbox identity")
		}

		return nil, errors.New("the signing CA issued a certificate with a sandbox identity mark that was not requested")
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
	ctx, span := otelx.Start(ctx, signingTracer, "SigningUseCase.GetTrustedRoot")
	defer span.End()

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
		keyIDSum := sha256.Sum256(chain[0].SubjectKeyId)
		keyID := hex.EncodeToString(keyIDSum[:])
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
			authorityKeyIDSum := sha256.Sum256(authority.CertChain[0].SubjectKeyId)
			authorityKeyID := hex.EncodeToString(authorityKeyIDSum[:])
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
	// sandboxIdentityIssuer adds the mark that lets the certificate sign a sandbox identity
	sandboxIdentityIssuer bool
}

var _ identity.Principal = (*chainloopPrincipal)(nil)

func newChainloopPrincipal(orgID string, sandboxIdentityIssuer bool) *chainloopPrincipal {
	return &chainloopPrincipal{orgID: orgID, sandboxIdentityIssuer: sandboxIdentityIssuer}
}

func (p *chainloopPrincipal) Name(_ context.Context) string {
	return p.orgID
}

func (p *chainloopPrincipal) Embed(_ context.Context, cert *x509.Certificate) error {
	// no op.
	// TODO: Chainloop might have their own private enterprise number with the Internet Assigned Numbers Authority
	// 		 to embed its own identity information in the resulting certificate
	cert.Subject = pkix.Name{Organization: []string{p.orgID}}
	if p.sandboxIdentityIssuer {
		cert.Subject.OrganizationalUnit = []string{sandboxidentity.CertificateMark}
	}

	return nil
}

// SandboxIdentity is the verified identity of a Chainloop sandbox, as recorded on its workflow run.
type SandboxIdentity struct {
	// SandboxRun identifies the sandbox run in the platform
	SandboxRun string `json:"sandbox_run,omitempty"`
	// ChainloopTokenID is the API token that the identity names
	ChainloopTokenID uuid.UUID `json:"chainloop_token_id"`
	// IssuedAt is the time of signing
	IssuedAt time.Time `json:"issued_at"`
	// Data is the data that the platform gives to the attestation
	Data *sandboxidentity.Data `json:"data,omitempty"`
}

// HoldsCommitVerification reports whether the identity holds the commit verification that an
// attestation records: the same full commit hash and status name. When the identity has a
// signature algorithm, the attestation must record the same one. Without it, the attestation takes
// the algorithm from the signature of the commit, so it is not compared.
func (i *SandboxIdentity) HoldsCommitVerification(v chainloop.GitHeadVerification) bool {
	if i == nil || i.Data == nil || i.Data.CommitVerification == nil || i.Data.CommitVerification.Verification == nil {
		return false
	}

	cv := i.Data.CommitVerification
	if v.Commit == "" || !strings.EqualFold(cv.Commit, v.Commit) || cv.Verification.Status.String() != v.Status {
		return false
	}

	return cv.Verification.SignatureAlgorithm == "" || cv.Verification.SignatureAlgorithm == v.SignatureAlgorithm
}

// VerifySandboxIdentity verifies the identity token of a Chainloop sandbox against the certificate
// authorities of the control plane. The identity must be for the expected attestation, and the
// caller must use the Chainloop token that the identity names.
func (s *SigningUseCase) VerifySandboxIdentity(ctx context.Context, raw string, expected *sandboxidentity.Expected, callerTokenID uuid.UUID) (*SandboxIdentity, error) {
	ctx, span := otelx.Start(ctx, signingTracer, "SigningUseCase.VerifySandboxIdentity")
	defer span.End()

	if s.CAs == nil {
		return nil, NewErrNotImplemented("CAs not initialized")
	}

	tr, err := s.trustedRootCertificates(ctx)
	if err != nil {
		return nil, err
	}

	id, err := sandboxidentity.Verify(raw, tr, expected)
	if err != nil {
		return nil, NewErrValidation(err)
	}

	tokenID, err := uuid.Parse(id.Claims.ChainloopTokenID)
	if err != nil || tokenID != callerTokenID {
		return nil, NewErrValidationStr("the identity names another Chainloop token")
	}

	return &SandboxIdentity{
		SandboxRun:       id.Claims.SandboxRun,
		ChainloopTokenID: tokenID,
		IssuedAt:         id.Claims.IssuedAt.Time,
		Data:             id.Claims.Data,
	}, nil
}

// trustedRootCertificates returns the root chains of the certificate authorities, keyed as in the
// trusted root that the CLI gets.
func (s *SigningUseCase) trustedRootCertificates(ctx context.Context) (*verifier.TrustedRoot, error) {
	tr := &verifier.TrustedRoot{Keys: make(map[string][]*x509.Certificate)}
	for _, auth := range s.CAs.GetAuthorities() {
		chain, err := auth.GetRootChain(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting root chain: %w", err)
		}

		if len(chain) == 0 {
			continue
		}

		keyIDSum := sha256.Sum256(chain[0].SubjectKeyId)
		tr.Keys[hex.EncodeToString(keyIDSum[:])] = chain
	}

	return tr, nil
}
