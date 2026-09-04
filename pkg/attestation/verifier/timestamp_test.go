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

package verifier

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/digitorus/timestamp"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protodsse "github.com/sigstore/protobuf-specs/gen/pb-go/dsse"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSignature is the DSSE signature the timestamps in these tests are taken
// over. It is deliberately not valid base64 so that VerifyTimestamps' legacy
// double-encoding workaround leaves it untouched.
var testSignature = []byte("!chainloop-test-signature!")

func TestVerifyTimestamps_SignerClassification(t *testing.T) {
	root, rootKey := newTestCA(t)
	responder := newTestTSALeaf(t, "Chainloop Test Timestamp Responder 1", root, rootKey)
	// The same authority after rotating its responder certificate: same root,
	// different leaf. This is what a DigiCert cutover looks like to us.
	rotated := newTestTSALeaf(t, "Chainloop Test Timestamp Responder 2", root, rootKey)

	otherRoot, otherRootKey := newTestCA(t)
	unrelated := newTestTSALeaf(t, "Unrelated Timestamp Responder", otherRoot, otherRootKey)

	// A response signed by `responder` over the test signature.
	tsr := createTestTimestamp(t, responder, testSignature)
	// A response signed by `responder` but covering different content: the
	// timestamp does not apply to this signature, which is an attestation fault.
	tsrOverOtherContent := createTestTimestamp(t, responder, []byte("!other-content!"))

	cases := []struct {
		name string
		// authorities maps an authority name to its pinned chain, leaf first.
		authorities map[string][]*x509.Certificate
		tsr         []byte
		// expectSentinel nil means verification is expected to succeed.
		expectSentinel error
		// expectTrustConfig asserts the failure is attributed to our own trust
		// configuration rather than to the attestation.
		expectTrustConfig bool
	}{
		{
			name:        "pinned chain matches the responder",
			authorities: map[string][]*x509.Certificate{"test-tsa": {responder.cert, root.cert}},
			tsr:         tsr,
		},
		{
			name:              "upstream responder rotated ahead of our pinned chain",
			authorities:       map[string][]*x509.Certificate{"test-tsa": {rotated.cert, root.cert}},
			tsr:               tsr,
			expectSentinel:    ErrTSASignerNotTrusted,
			expectTrustConfig: true,
		},
		{
			name:              "signer belongs to no configured authority",
			authorities:       map[string][]*x509.Certificate{"other-tsa": {unrelated.cert, otherRoot.cert}},
			tsr:               tsr,
			expectSentinel:    ErrTSASignerNotTrusted,
			expectTrustConfig: true,
		},
		{
			name: "one of several authorities matches the responder",
			authorities: map[string][]*x509.Certificate{
				"rotated-tsa": {rotated.cert, root.cert},
				"other-tsa":   {unrelated.cert, otherRoot.cert},
				"test-tsa":    {responder.cert, root.cert},
			},
			tsr: tsr,
		},
		{
			name:        "timestamp does not cover the signature",
			authorities: map[string][]*x509.Certificate{"test-tsa": {responder.cert, root.cert}},
			tsr:         tsrOverOtherContent,
			// The signer is trusted, so the failure is the attestation's own.
			expectSentinel:    ErrTSAResponseInvalid,
			expectTrustConfig: false,
		},
		{
			name:           "unparseable response with a trusted authority",
			authorities:    map[string][]*x509.Certificate{"test-tsa": {responder.cert, root.cert}},
			tsr:            []byte("not-a-valid-tsr"),
			expectSentinel: ErrTSAResponseInvalid,
			// We cannot attribute a malformed response to our configuration.
			expectTrustConfig: false,
		},
		{
			name:              "no authorities configured",
			authorities:       nil,
			tsr:               tsr,
			expectSentinel:    ErrNoTSARootsConfigured,
			expectTrustConfig: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyTimestamps(
				testBundleWithTimestamp(t, testSignature, tc.tsr),
				&TrustedRoot{TimestampAuthorities: tc.authorities},
			)

			if tc.expectSentinel == nil {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.expectSentinel), "expected %v, got: %v", tc.expectSentinel, err)
			assert.Equal(t, tc.expectTrustConfig, IsTrustConfigError(err), "unexpected fault attribution for: %v", err)
		})
	}
}

// TestVerifyTimestamps_RotatedSignerErrorNamesAuthority checks the rotation
// error is self-diagnosing: it must say which authority was tried and which
// leaf we expected, so the chain to rotate is identifiable from the message.
func TestVerifyTimestamps_RotatedSignerErrorNamesAuthority(t *testing.T) {
	root, rootKey := newTestCA(t)
	responder := newTestTSALeaf(t, "Chainloop Test Timestamp Responder 1", root, rootKey)
	rotated := newTestTSALeaf(t, "Chainloop Test Timestamp Responder 2", root, rootKey)

	err := VerifyTimestamps(
		testBundleWithTimestamp(t, testSignature, createTestTimestamp(t, responder, testSignature)),
		&TrustedRoot{TimestampAuthorities: map[string][]*x509.Certificate{
			"prod-tsa": {rotated.cert, root.cert},
		}},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "prod-tsa")
	assert.Contains(t, err.Error(), rotated.cert.Subject.CommonName)
}

func TestIsTrustConfigError(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		expect bool
	}{
		{name: "nil", err: nil, expect: false},
		{name: "rotated signer", err: ErrTSASignerNotTrusted, expect: true},
		{name: "wrapped rotated signer", err: errors.Join(errors.New("boom"), ErrTSASignerNotTrusted), expect: true},
		{name: "no roots configured", err: ErrNoTSARootsConfigured, expect: true},
		{name: "invalid response", err: ErrTSAResponseInvalid, expect: false},
		{name: "timestamp outside TSA validity", err: ErrTimestampOutsideTSAValidity, expect: false},
		{name: "signing cert not valid at timestamp", err: ErrSigningCertNotValidAtTimestamp, expect: false},
		{name: "unrelated", err: errors.New("boom"), expect: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, IsTrustConfigError(tc.err))
		})
	}
}

// testKeyPair is a certificate together with the key that signs with it.
type testKeyPair struct {
	cert *x509.Certificate
	key  *rsa.PrivateKey
}

// newTestCA builds a self-signed CA to act as a timestamp authority root.
func newTestCA(t *testing.T) (*testKeyPair, *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber:          newTestSerial(t),
		Subject:               pkix.Name{CommonName: "Chainloop Test Timestamp Root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	return &testKeyPair{cert: signTestCert(t, tmpl, tmpl, key.Public(), key), key: key}, key
}

// newTestTSALeaf builds an end-entity timestamping certificate issued by the
// given CA. RFC 3161 requires the extended key usage extension to be critical
// and to carry timestamping only, which x509.CreateCertificate does not do on
// its own, so the extension is written explicitly.
func newTestTSALeaf(t *testing.T, commonName string, ca *testKeyPair, caKey *rsa.PrivateKey) *testKeyPair {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// id-kp-timeStamping, per RFC 5280.
	eku, err := asn1.Marshal([]asn1.ObjectIdentifier{{1, 3, 6, 1, 5, 5, 7, 3, 8}})
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: newTestSerial(t),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{{
			Id:       asn1.ObjectIdentifier{2, 5, 29, 37}, // extended key usage
			Critical: true,
			Value:    eku,
		}},
		BasicConstraintsValid: true,
	}

	return &testKeyPair{cert: signTestCert(t, tmpl, ca.cert, key.Public(), caKey), key: key}
}

func signTestCert(t *testing.T, tmpl, parent *x509.Certificate, pub crypto.PublicKey, signer crypto.Signer) *x509.Certificate {
	t.Helper()

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, signer)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return cert
}

func newTestSerial(t *testing.T) *big.Int {
	t.Helper()

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	return serial
}

// createTestTimestamp issues an RFC3161 response over content, signed by the
// given responder. Certificates are left out of the response, matching the
// timestamp requests Chainloop makes.
func createTestTimestamp(t *testing.T, responder *testKeyPair, content []byte) []byte {
	t.Helper()

	digest := sha256.Sum256(content)
	ts := &timestamp.Timestamp{
		HashAlgorithm: crypto.SHA256,
		HashedMessage: digest[:],
		Time:          time.Now(),
		Policy:        asn1.ObjectIdentifier{1, 2, 3, 4, 1},
	}

	tsr, err := ts.CreateResponseWithOpts(responder.cert, responder.key, crypto.SHA256)
	require.NoError(t, err)

	return tsr
}

// testBundleWithTimestamp builds the minimum bundle VerifyTimestamps needs: a
// DSSE envelope carrying signature and one RFC3161 timestamp.
func testBundleWithTimestamp(t *testing.T, signature, tsr []byte) *sigstorebundle.Bundle {
	t.Helper()

	return &sigstorebundle.Bundle{Bundle: &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle+json;version=0.3",
		VerificationMaterial: &protobundle.VerificationMaterial{
			TimestampVerificationData: &protobundle.TimestampVerificationData{
				Rfc3161Timestamps: []*protocommon.RFC3161SignedTimestamp{{SignedTimestamp: tsr}},
			},
		},
		Content: &protobundle.Bundle_DsseEnvelope{DsseEnvelope: &protodsse.Envelope{
			Payload:     []byte(`{"_type":"https://in-toto.io/Statement/v1"}`),
			PayloadType: "application/vnd.in-toto+json",
			Signatures:  []*protodsse.Signature{{Sig: signature}},
		}},
	}}
}
