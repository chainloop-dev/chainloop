//
// Copyright 2026 The Chainloop Authors.
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

// Package sandboxidentity defines the identity token that the Chainloop platform gives to each
// Chainloop sandbox that it spawns, and how to sign and verify it.
//
// The token is a JWT signed with a new key for each sandbox. The key has a short-lived certificate
// from the control plane CA, and the certificate chain is in the x5c header. The CA marks the
// certificate only for a caller that holds the dedicated permission, so only the platform can sign
// an identity. The token has no expiry of its own: the control plane accepts it only together with
// the Chainloop token that it names, and that token expires.
package sandboxidentity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/chainloop-dev/chainloop/pkg/attestation/verifier"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// EnvVar holds the identity token in the environment of the sandbox.
	EnvVar = "CHAINLOOP_SANDBOX_IDENTITY"

	// CertificateMark is the value in the organizational unit of the certificate subject that
	// says that the certificate can sign an identity. Only the CA writes the subject.
	CertificateMark = "chainloop-sandbox-identity"

	// MaxClockSkew is the clock difference allowed between the platform and the CA when the time
	// of signing is compared with the certificate lifetime.
	MaxClockSkew = time.Minute

	x5cHeader = "x5c"
)

// ErrInvalidIdentity is the error for an identity token that does not verify.
var ErrInvalidIdentity = errors.New("invalid sandbox identity")

var (
	validMethods   = []string{jwt.SigningMethodES256.Alg(), jwt.SigningMethodES384.Alg(), jwt.SigningMethodES512.Alg()}
	fullCommitHash = regexp.MustCompile(`^([0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)
)

// Claims are the claims of an identity token.
type Claims struct {
	// RegisteredClaims holds the time of signing (iat). The token has no expiry.
	jwt.RegisteredClaims
	// OrganizationID is the organization of the attestation.
	OrganizationID string `json:"org"`
	// ProjectID is the project of the attested workflow.
	ProjectID string `json:"project"`
	// WorkflowID is the attested workflow.
	WorkflowID string `json:"workflow"`
	// SandboxRun identifies the sandbox run in the platform, so that a reader can trace an
	// attestation to the run that made it.
	SandboxRun string `json:"sandbox_run,omitempty"`
	// ChainloopTokenID is the ID of the Chainloop API token that the platform gives to the sandbox.
	// The control plane accepts the identity only from this token.
	ChainloopTokenID string `json:"chainloop_token"`
	// Data is the data that the platform gives to the attestation.
	Data *Data `json:"data,omitempty"`
}

// Data is the data of a sandbox run that the attestation can trust.
type Data struct {
	// CommitVerification is the verification of the attested commit by its SCM provider.
	CommitVerification *CommitVerification `json:"commit_verification,omitempty"`
}

// CommitVerification is a commit verification result and the commit that it applies to.
type CommitVerification struct {
	// Commit is the full hash of the verified commit.
	Commit string `json:"commit"`
	// Verification is the result of the verification.
	Verification *commitverification.CommitVerification `json:"verification"`
}

// Expected names the attestation that an identity must be for.
type Expected struct {
	// OrganizationID is the organization of the attestation. When empty, it is not compared: the
	// workflow belongs to one organization, and the CLI does not know the organization ID.
	OrganizationID string
	// ProjectID is the project of the attestation. Required.
	ProjectID string
	// WorkflowID is the workflow of the attestation. Required.
	WorkflowID string
}

// Identity is an identity token that verified.
type Identity struct {
	// Claims are the claims of the token.
	Claims *Claims
	// Certificate is the certificate that signed the token.
	Certificate *x509.Certificate
}

// CommitVerification returns the commit verification of the identity, only when it is for the
// full commitHash. It returns nil for another or an abbreviated hash.
func (i *Identity) CommitVerification(commitHash string) *commitverification.CommitVerification {
	if i == nil || i.Claims == nil || i.Claims.Data == nil || i.Claims.Data.CommitVerification == nil || commitHash == "" {
		return nil
	}

	cv := i.Claims.Data.CommitVerification
	if !strings.EqualFold(cv.Commit, commitHash) {
		return nil
	}

	return cv.Verification
}

// HasMark reports whether the certificate can sign an identity.
func HasMark(cert *x509.Certificate) bool {
	return cert != nil && slices.Contains(cert.Subject.OrganizationalUnit, CertificateMark)
}

// Sign signs the claims with the key, and puts the certificate chain, leaf first, in the header.
func Sign(key *ecdsa.PrivateKey, chain []*x509.Certificate, claims *Claims) (string, error) {
	if key == nil {
		return "", errors.New("a signing key is required")
	}

	if len(chain) == 0 {
		return "", errors.New("a certificate chain is required")
	}

	if err := claims.validate(); err != nil {
		return "", err
	}

	var method jwt.SigningMethod
	switch key.Curve {
	case elliptic.P256():
		method = jwt.SigningMethodES256
	case elliptic.P384():
		method = jwt.SigningMethodES384
	case elliptic.P521():
		method = jwt.SigningMethodES512
	default:
		return "", fmt.Errorf("unsupported curve %s", key.Curve.Params().Name)
	}

	token := jwt.NewWithClaims(method, claims)
	token.Header[x5cHeader] = encodeChain(chain)

	return token.SignedString(key)
}

// Verify verifies an identity token against the trusted root of the control plane, and checks
// that it is for the expected attestation. Any error wraps ErrInvalidIdentity.
func Verify(raw string, tr *verifier.TrustedRoot, expected *Expected) (*Identity, error) {
	id, err := verify(raw, tr, expected)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidIdentity, err)
	}

	return id, nil
}

func verify(raw string, tr *verifier.TrustedRoot, expected *Expected) (*Identity, error) {
	if tr == nil {
		return nil, errors.New("no trusted root")
	}

	if expected == nil || expected.ProjectID == "" || expected.WorkflowID == "" {
		return nil, errors.New("the expected project and workflow are required")
	}

	var leaf *x509.Certificate
	keyFunc := func(token *jwt.Token) (any, error) {
		chain, err := decodeChain(token.Header[x5cHeader])
		if err != nil {
			return nil, err
		}

		leaf = chain[0]
		// The chain in the header is not trusted. Only the trusted root validates the leaf.
		if _, err := verifier.ValidateCertificate(leaf, tr); err != nil {
			return nil, err
		}

		if !HasMark(leaf) {
			return nil, errors.New("the certificate cannot sign an identity")
		}

		return leaf.PublicKey, nil
	}

	claims := &Claims{}
	if _, err := jwt.NewParser(jwt.WithValidMethods(validMethods)).ParseWithClaims(raw, claims, keyFunc); err != nil {
		return nil, err
	}

	if err := claims.validate(); err != nil {
		return nil, err
	}

	// The CA writes the organization of the caller in the certificate subject.
	if len(leaf.Subject.Organization) != 1 || leaf.Subject.Organization[0] != claims.OrganizationID {
		return nil, errors.New("the certificate is for another organization")
	}

	// The certificate is short-lived and the key signs only once, so the time of signing must be
	// inside the certificate lifetime.
	iat := claims.IssuedAt.Time
	if iat.Before(leaf.NotBefore.Add(-MaxClockSkew)) || iat.After(leaf.NotAfter.Add(MaxClockSkew)) {
		return nil, errors.New("the time of signing is outside the certificate lifetime")
	}

	if expected.OrganizationID != "" && claims.OrganizationID != expected.OrganizationID {
		return nil, errors.New("the identity is for another organization")
	}

	if claims.ProjectID != expected.ProjectID {
		return nil, errors.New("the identity is for another project")
	}

	if claims.WorkflowID != expected.WorkflowID {
		return nil, errors.New("the identity is for another workflow")
	}

	return &Identity{Claims: claims, Certificate: leaf}, nil
}

func (c *Claims) validate() error {
	if c == nil {
		return errors.New("no claims")
	}

	if c.IssuedAt == nil {
		return errors.New("the time of signing is required")
	}

	if c.OrganizationID == "" || c.ProjectID == "" || c.WorkflowID == "" {
		return errors.New("the organization, the project and the workflow are required")
	}

	if c.ChainloopTokenID == "" {
		return errors.New("the Chainloop token is required")
	}

	if c.Data == nil || c.Data.CommitVerification == nil {
		return nil
	}

	cv := c.Data.CommitVerification
	if !fullCommitHash.MatchString(cv.Commit) {
		return errors.New("the commit verification needs a full commit hash")
	}

	if cv.Verification == nil || cv.Verification.Status == commitverification.VerificationStatusUnspecified {
		return errors.New("the commit verification has no result")
	}

	return nil
}

func encodeChain(chain []*x509.Certificate) []string {
	encoded := make([]string, 0, len(chain))
	for _, cert := range chain {
		encoded = append(encoded, base64.StdEncoding.EncodeToString(cert.Raw))
	}

	return encoded
}

func decodeChain(header any) ([]*x509.Certificate, error) {
	values, ok := header.([]any)
	if !ok || len(values) == 0 {
		return nil, errors.New("the token has no certificate chain")
	}

	chain := make([]*x509.Certificate, 0, len(values))
	for _, v := range values {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("invalid certificate chain")
		}

		der, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("decoding the certificate chain: %w", err)
		}

		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("parsing the certificate chain: %w", err)
		}

		chain = append(chain, cert)
	}

	return chain, nil
}
