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

package sandboxidentity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/chainloop-dev/chainloop/pkg/attestation/verifier"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testOrg      = "5d0b4b7c-4b2e-4a51-9a4b-2f0a2b1f6e10"
	testProject  = "6e1c5c8d-5c3f-4b62-8b5c-3f1b3c2f7f21"
	testWorkflow = "7f2d6d9e-6d4f-4c73-9c6d-4f2c4d3f8f32"
	testToken    = "8f3e7eaf-7e5f-4d84-8d7e-5f3d5e4f9f43"
	testCommit   = "0123456789abcdef0123456789abcdef01234567"
)

// testCA is a certificate authority that issues short-lived code signing certificates, as the
// control plane CA does.
type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
		SubjectKeyId:          []byte("test-root-key-id"),
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return &testCA{cert: cert, key: key}
}

func (ca *testCA) trustedRoot() *verifier.TrustedRoot {
	sum := sha256.Sum256(ca.cert.SubjectKeyId)

	return &verifier.TrustedRoot{Keys: map[string][]*x509.Certificate{hex.EncodeToString(sum[:]): {ca.cert}}}
}

// issue returns a leaf certificate for a new key, with the given subject and lifetime.
func (ca *testCA) issue(t *testing.T, subject pkix.Name, notBefore time.Time) (*ecdsa.PrivateKey, *x509.Certificate) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      subject,
		NotBefore:    notBefore,
		NotAfter:     notBefore.Add(10 * time.Minute),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return key, cert
}

func markedSubject(org string) pkix.Name {
	return pkix.Name{Organization: []string{org}, OrganizationalUnit: []string{CertificateMark}}
}

func validClaims(iat time.Time) *Claims {
	return &Claims{
		RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(iat)},
		OrganizationID:   testOrg,
		ProjectID:        testProject,
		WorkflowID:       testWorkflow,
		SandboxRun:       "sandbox-run-1",
		ChainloopTokenID: testToken,
		Data: &Data{CommitVerification: &CommitVerification{
			Commit: testCommit,
			Verification: &commitverification.CommitVerification{
				Attempted: true, Status: commitverification.VerificationStatusVerified, Platform: "github",
			},
		}},
	}
}

func validExpected() *Expected {
	return &Expected{OrganizationID: testOrg, ProjectID: testProject, WorkflowID: testWorkflow}
}

func TestSignAndVerify(t *testing.T) {
	ca := newTestCA(t)
	now := time.Now()
	key, leaf := ca.issue(t, markedSubject(testOrg), now.Add(-time.Minute))

	raw, err := Sign(key, []*x509.Certificate{leaf, ca.cert}, validClaims(now))
	require.NoError(t, err)

	id, err := Verify(raw, ca.trustedRoot(), validExpected())
	require.NoError(t, err)

	assert.Equal(t, testToken, id.Claims.ChainloopTokenID)
	assert.Equal(t, "sandbox-run-1", id.Claims.SandboxRun)
	assert.Equal(t, leaf.SerialNumber, id.Certificate.SerialNumber)

	t.Run("the commit verification applies only to its full commit hash", func(t *testing.T) {
		got := id.CommitVerification(strings.ToUpper(testCommit))
		require.NotNil(t, got)
		assert.Equal(t, commitverification.VerificationStatusVerified, got.Status)

		assert.Nil(t, id.CommitVerification(testCommit[:12]))
		assert.Nil(t, id.CommitVerification("fedcba9876543210fedcba9876543210fedcba98"))
		assert.Nil(t, id.CommitVerification(""))
	})

	t.Run("the CLI does not know the organization", func(t *testing.T) {
		expected := validExpected()
		expected.OrganizationID = ""

		_, err := Verify(raw, ca.trustedRoot(), expected)
		assert.NoError(t, err)
	})
}

func TestVerifyRejects(t *testing.T) {
	ca := newTestCA(t)
	now := time.Now()
	notBefore := now.Add(-time.Minute)
	key, leaf := ca.issue(t, markedSubject(testOrg), notBefore)

	// sign does not check the claims, so that the verifier sees the invalid ones
	sign := func(t *testing.T, key *ecdsa.PrivateKey, chain []*x509.Certificate, claims *Claims) string {
		t.Helper()
		token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		token.Header[x5cHeader] = encodeChain(chain)
		raw, err := token.SignedString(key)
		require.NoError(t, err)
		return raw
	}

	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	unmarkedKey, unmarkedLeaf := ca.issue(t, pkix.Name{Organization: []string{testOrg}}, notBefore)

	otherCA := newTestCA(t)
	otherCAKey, otherCALeaf := otherCA.issue(t, markedSubject(testOrg), notBefore)

	otherOrgKey, otherOrgLeaf := ca.issue(t, markedSubject("another-org"), notBefore)

	testCases := []struct {
		name     string
		raw      func(t *testing.T) string
		expected *Expected
	}{
		{
			name: "a signature by another key",
			raw:  func(t *testing.T) string { return sign(t, otherKey, []*x509.Certificate{leaf}, validClaims(now)) },
		},
		{
			name: "a certificate without the mark",
			raw: func(t *testing.T) string {
				return sign(t, unmarkedKey, []*x509.Certificate{unmarkedLeaf}, validClaims(now))
			},
		},
		{
			name: "a certificate from an untrusted CA",
			raw: func(t *testing.T) string {
				return sign(t, otherCAKey, []*x509.Certificate{otherCALeaf}, validClaims(now))
			},
		},
		{
			name: "a certificate for another organization than the token",
			raw: func(t *testing.T) string {
				return sign(t, otherOrgKey, []*x509.Certificate{otherOrgLeaf}, validClaims(now))
			},
		},
		{
			name:     "another workflow",
			raw:      func(t *testing.T) string { return sign(t, key, []*x509.Certificate{leaf}, validClaims(now)) },
			expected: &Expected{OrganizationID: testOrg, ProjectID: testProject, WorkflowID: "another-workflow"},
		},
		{
			name:     "another project",
			raw:      func(t *testing.T) string { return sign(t, key, []*x509.Certificate{leaf}, validClaims(now)) },
			expected: &Expected{OrganizationID: testOrg, ProjectID: "another-project", WorkflowID: testWorkflow},
		},
		{
			name:     "another organization than the attestation",
			raw:      func(t *testing.T) string { return sign(t, key, []*x509.Certificate{leaf}, validClaims(now)) },
			expected: &Expected{OrganizationID: "another-org", ProjectID: testProject, WorkflowID: testWorkflow},
		},
		{
			name:     "no expected workflow",
			raw:      func(t *testing.T) string { return sign(t, key, []*x509.Certificate{leaf}, validClaims(now)) },
			expected: &Expected{OrganizationID: testOrg, ProjectID: testProject},
		},
		{
			name: "a time of signing before the certificate lifetime",
			raw: func(t *testing.T) string {
				return sign(t, key, []*x509.Certificate{leaf}, validClaims(notBefore.Add(-2*MaxClockSkew)))
			},
		},
		{
			name: "a time of signing after the certificate lifetime",
			raw: func(t *testing.T) string {
				return sign(t, key, []*x509.Certificate{leaf}, validClaims(leaf.NotAfter.Add(2*MaxClockSkew)))
			},
		},
		{
			name: "no time of signing",
			raw: func(t *testing.T) string {
				claims := validClaims(now)
				claims.IssuedAt = nil
				return sign(t, key, []*x509.Certificate{leaf}, claims)
			},
		},
		{
			name: "no certificate chain",
			raw: func(t *testing.T) string {
				raw, err := jwt.NewWithClaims(jwt.SigningMethodES256, validClaims(now)).SignedString(key)
				require.NoError(t, err)
				return raw
			},
		},
		{
			name: "a symmetric signature",
			raw: func(t *testing.T) string {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims(now))
				token.Header[x5cHeader] = encodeChain([]*x509.Certificate{leaf})
				raw, err := token.SignedString([]byte("secret"))
				require.NoError(t, err)
				return raw
			},
		},
		{
			name: "an abbreviated commit hash",
			raw: func(t *testing.T) string {
				claims := validClaims(now)
				claims.Data.CommitVerification.Commit = testCommit[:12]
				return sign(t, key, []*x509.Certificate{leaf}, claims)
			},
		},
		{
			name: "a commit verification without a status",
			raw: func(t *testing.T) string {
				claims := validClaims(now)
				claims.Data.CommitVerification.Verification.Status = commitverification.VerificationStatusUnspecified
				return sign(t, key, []*x509.Certificate{leaf}, claims)
			},
		},
		{
			name: "a commit verification without a result",
			raw: func(t *testing.T) string {
				claims := validClaims(now)
				claims.Data.CommitVerification.Verification = nil
				return sign(t, key, []*x509.Certificate{leaf}, claims)
			},
		},
		{
			name: "no Chainloop token",
			raw: func(t *testing.T) string {
				claims := validClaims(now)
				claims.ChainloopTokenID = ""
				return sign(t, key, []*x509.Certificate{leaf}, claims)
			},
		},
		{
			name: "a value that is not a token",
			raw:  func(_ *testing.T) string { return "not-a-token" },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expected := tc.expected
			if expected == nil {
				expected = validExpected()
			}

			id, err := Verify(tc.raw(t), ca.trustedRoot(), expected)
			require.ErrorIs(t, err, ErrInvalidIdentity)
			assert.Nil(t, id)
		})
	}

	t.Run("no trusted root", func(t *testing.T) {
		_, err := Verify(sign(t, key, []*x509.Certificate{leaf}, validClaims(now)), nil, validExpected())
		require.ErrorIs(t, err, ErrInvalidIdentity)
	})
}

func TestSignRejectsInvalidClaims(t *testing.T) {
	ca := newTestCA(t)
	key, leaf := ca.issue(t, markedSubject(testOrg), time.Now().Add(-time.Minute))

	claims := validClaims(time.Now())
	claims.WorkflowID = ""

	_, err := Sign(key, []*x509.Certificate{leaf}, claims)
	require.Error(t, err)

	_, err = Sign(key, nil, validClaims(time.Now()))
	require.Error(t, err)
}

func TestHasMark(t *testing.T) {
	assert.True(t, HasMark(&x509.Certificate{Subject: markedSubject(testOrg)}))
	assert.False(t, HasMark(&x509.Certificate{Subject: pkix.Name{Organization: []string{testOrg}}}))
	assert.False(t, HasMark(nil))
}
