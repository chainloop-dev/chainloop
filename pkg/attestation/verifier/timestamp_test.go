//
// Copyright 2025 The Chainloop Authors.
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
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"testing"
	"time"

	"github.com/digitorus/pkcs7"
	"github.com/digitorus/timestamp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyTimestampAtTime(t *testing.T) {
	// Create a CA (root) certificate
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Root CA"},
		NotBefore:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:     time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:         true,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	// signatureToTimestamp is the artifact being timestamped
	signatureToTimestamp := []byte("test-signature-data")

	// createTSACertAndResponse creates a TSA leaf cert with the given validity window,
	// then generates a signed timestamp response at the given timestamp time.
	createTSACertAndResponse := func(t *testing.T, certNotBefore, certNotAfter, tsTime time.Time) (*x509.Certificate, []byte) {
		t.Helper()

		tsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		tsaTemplate := &x509.Certificate{
			SerialNumber: big.NewInt(2),
			Subject:      pkix.Name{CommonName: "Test TSA"},
			NotBefore:    certNotBefore,
			NotAfter:     certNotAfter,
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
		}
		tsaDER, err := x509.CreateCertificate(rand.Reader, tsaTemplate, caTemplate, &tsaKey.PublicKey, caKey)
		require.NoError(t, err)
		tsaCert, err := x509.ParseCertificate(tsaDER)
		require.NoError(t, err)

		// Build a timestamp token (RFC 3161)
		h := crypto.SHA256.New()
		h.Write(signatureToTimestamp)
		hashedMessage := h.Sum(nil)

		tsrBytes := buildTimestampResponse(t, tsaCert, tsaKey, hashedMessage, tsTime)
		return tsaCert, tsrBytes
	}

	cases := []struct {
		name         string
		certNotBefore time.Time
		certNotAfter  time.Time
		tsTime        time.Time
		expectErr     string
	}{
		{
			name:          "valid: timestamp within cert validity",
			certNotBefore: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			certNotAfter:  time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
			tsTime:        time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "valid: cert expired now but was valid at timestamp time",
			certNotBefore: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			certNotAfter:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			tsTime:        time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "invalid: timestamp before cert validity",
			certNotBefore: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			certNotAfter:  time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
			tsTime:        time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expectErr:     "verifying TSA certificate chain",
		},
		{
			name:          "invalid: timestamp after cert validity",
			certNotBefore: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			certNotAfter:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			tsTime:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			expectErr:     "verifying TSA certificate chain",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tsaCert, tsrBytes := createTSACertAndResponse(t, tc.certNotBefore, tc.certNotAfter, tc.tsTime)

			ts, err := verifyTimestampAtTime(tsrBytes, signatureToTimestamp, tsaCert, nil, []*x509.Certificate{caCert})
			if tc.expectErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErr)
				return
			}
			require.NoError(t, err)
			assert.False(t, ts.Time.IsZero())
		})
	}

	t.Run("invalid: hash mismatch", func(t *testing.T) {
		tsaCert, tsrBytes := createTSACertAndResponse(t,
			time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		)
		wrongSignature := []byte("wrong-signature-data")
		_, err := verifyTimestampAtTime(tsrBytes, wrongSignature, tsaCert, nil, []*x509.Certificate{caCert})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hashed message mismatch")
	})
}

// buildTimestampResponse creates a minimal RFC 3161 timestamp response for testing.
func buildTimestampResponse(t *testing.T, tsaCert *x509.Certificate, tsaKey *rsa.PrivateKey, hashedMessage []byte, tsTime time.Time) []byte {
	t.Helper()

	// Build the TSTInfo (timestamp token info)
	tstInfo := struct {
		Version        int
		Policy         asn1.ObjectIdentifier
		MessageImprint struct {
			HashAlgorithm pkix.AlgorithmIdentifier
			HashedMessage []byte
		}
		SerialNumber *big.Int
		GenTime      time.Time `asn1:"generalized"`
	}{
		Version: 1,
		Policy:  asn1.ObjectIdentifier{1, 2, 3, 4},
		MessageImprint: struct {
			HashAlgorithm pkix.AlgorithmIdentifier
			HashedMessage []byte
		}{
			HashAlgorithm: pkix.AlgorithmIdentifier{Algorithm: asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}}, // SHA-256
			HashedMessage: hashedMessage,
		},
		SerialNumber: big.NewInt(100),
		GenTime:      tsTime,
	}
	tstInfoDER, err := asn1.Marshal(tstInfo)
	require.NoError(t, err)

	// Wrap in a PKCS7 signed data structure
	signedData, err := pkcs7.NewSignedData(tstInfoDER)
	require.NoError(t, err)
	// Use OID for id-smime-ct-TSTInfo
	signedData.SetContentType(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 1, 4})
	err = signedData.AddSigner(tsaCert, tsaKey, pkcs7.SignerInfoConfig{})
	require.NoError(t, err)
	p7DER, err := signedData.Finish()
	require.NoError(t, err)

	// Wrap in a TimeStampResp structure
	tsResp := struct {
		Status struct {
			Status int
		}
		TimeStampToken asn1.RawValue
	}{
		Status: struct{ Status int }{Status: 0}, // granted
		TimeStampToken: asn1.RawValue{
			Class:      asn1.ClassUniversal,
			Tag:        asn1.TagSequence,
			IsCompound: true,
			Bytes:      p7DER,
		},
	}

	// Re-parse the PKCS7 to get the full DER with the outer SEQUENCE tag
	tsRespBytes, err := asn1.Marshal(tsResp)
	require.NoError(t, err)

	// Verify our test fixture is valid by parsing it
	_, err = timestamp.ParseResponse(tsRespBytes)
	require.NoError(t, err, "test timestamp response should be parseable")

	return tsRespBytes
}
