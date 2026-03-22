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

package signserver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers -----------------------------------------------------------------

func generateSelfSignedCertPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func generatePrivateKeyPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

// generateEncryptedPrivateKeyPEM creates a PEM-encrypted private key (RFC1423/legacy format).
//
//nolint:staticcheck
func generateEncryptedPrivateKeyPEM(t *testing.T, password string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	// x509.EncryptPEMBlock is deprecated but remains the standard way to test
	// the RFC1423 decryption path exercised by privateKeyFromPem.
	block, err := x509.EncryptPEMBlock(rand.Reader, "PRIVATE KEY", der, []byte(password), x509.PEMCipherAES128) //nolint:staticcheck
	require.NoError(t, err)
	return pem.EncodeToMemory(block)
}

// writePEMToTempFile writes a raw PEM block to a temp file and returns its path.
func writePEMToTempFile(t *testing.T, blockType string, der []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.pem")
	require.NoError(t, err)
	defer f.Close()
	err = pem.Encode(f, &pem.Block{Type: blockType, Bytes: der})
	require.NoError(t, err)
	return f.Name()
}

// tlsServerCAFile starts a TLS httptest server and returns it together with a
// temp-file path containing its CA certificate. Caller is responsible for
// calling srv.Close().
func tlsServerWithCA(t *testing.T, handler http.Handler) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	caPath := writePEMToTempFile(t, "CERTIFICATE", srv.TLS.Certificates[0].Certificate[0])
	return srv, caPath
}

// --- TestParseKeyReference ---------------------------------------------------

func TestParseKeyReference(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantHost   string
		wantWorker string
		wantErr    bool
	}{
		{
			name:       "valid reference",
			input:      "signserver://myhost.com/CMSSigner",
			wantHost:   "myhost.com",
			wantWorker: "CMSSigner",
		},
		{
			name:       "valid reference with port",
			input:      "signserver://myhost.com:8443/PlainSigner",
			wantHost:   "myhost.com:8443",
			wantWorker: "PlainSigner",
		},
		{
			name:    "missing worker segment",
			input:   "signserver://myhost.com",
			wantErr: true,
		},
		{
			name:    "too many path segments",
			input:   "signserver://myhost.com/worker/extra",
			wantErr: true,
		},
		{
			name:    "no scheme separator",
			input:   "noscheme",
			wantErr: true,
		},
		{
			// ParseKeyReference only validates structure, not the scheme prefix.
			// Scheme enforcement (signserver://) is the caller's responsibility (GetSigner).
			name:       "non-signserver scheme parses successfully",
			input:      "https://myhost.com/worker",
			wantHost:   "myhost.com",
			wantWorker: "worker",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, worker, err := ParseKeyReference(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantHost, host)
			assert.Equal(t, tc.wantWorker, worker)
		})
	}
}

// --- TestNewSigner_WithOptions ------------------------------------------------

func TestNewSigner_WithOptions(t *testing.T) {
	s := NewSigner("myhost.com", "CMSSigner",
		WithCAPath("/ca.pem"),
		WithClientCertPath("/cert.pem"),
		WithClientCertPass("secret"),
	)
	assert.Equal(t, "myhost.com", s.host)
	assert.Equal(t, "CMSSigner", s.worker)
	assert.Equal(t, "/ca.pem", s.caPath)
	assert.Equal(t, "/cert.pem", s.clientCertPath)
	assert.Equal(t, "secret", s.clientCertPass)
}

func TestNewSigner_Defaults(t *testing.T) {
	s := NewSigner("host", "worker")
	assert.Empty(t, s.caPath)
	assert.Empty(t, s.clientCertPath)
	assert.Empty(t, s.clientCertPass)
}

// --- TestPublicKey -----------------------------------------------------------

func TestPublicKey_NotSupported(t *testing.T) {
	s := NewSigner("host", "worker")
	key, err := s.PublicKey()
	assert.Error(t, err)
	assert.Nil(t, key)
}

// --- TestSignMessage ---------------------------------------------------------

func TestSignMessage_Success(t *testing.T) {
	var gotWorker string
	var gotFileName string
	var gotFileContent []byte

	srv, caPath := tlsServerWithCA(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseMultipartForm(1<<20))
		gotWorker = r.FormValue("workerName")
		f, fh, err := r.FormFile("file")
		require.NoError(t, err)
		gotFileName = fh.Filename
		gotFileContent, err = io.ReadAll(f)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fakesignature"))
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "https://")
	s := NewSigner(host, "CMSSigner", WithCAPath(caPath))

	sig, err := s.SignMessage(strings.NewReader("hello attestation"))
	require.NoError(t, err)
	assert.Equal(t, []byte("fakesignature"), sig)
	assert.Equal(t, "CMSSigner", gotWorker)
	assert.Equal(t, "attestation.json", gotFileName)
	assert.Equal(t, []byte("hello attestation"), gotFileContent)
}

func TestSignMessage_Non200Status(t *testing.T) {
	srv, caPath := tlsServerWithCA(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "https://")
	s := NewSigner(host, "CMSSigner", WithCAPath(caPath))

	_, err := s.SignMessage(strings.NewReader("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestSignMessage_NetworkError(t *testing.T) {
	srv, caPath := tlsServerWithCA(t, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	host := strings.TrimPrefix(srv.URL, "https://")
	srv.Close() // closed before the call

	s := NewSigner(host, "CMSSigner", WithCAPath(caPath))
	_, err := s.SignMessage(strings.NewReader("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send request")
}

func TestSignMessage_InvalidCAPath(t *testing.T) {
	s := NewSigner("host", "worker", WithCAPath("/nonexistent/ca.pem"))
	_, err := s.SignMessage(strings.NewReader("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read ca cert")
}

// --- TestCertFromPem ---------------------------------------------------------

func TestCertFromPem(t *testing.T) {
	cert1 := generateSelfSignedCertPEM(t)
	cert2 := generateSelfSignedCertPEM(t)
	keyOnly := generatePrivateKeyPEM(t)

	tests := []struct {
		name      string
		input     []byte
		wantCerts int
	}{
		{"single certificate", cert1, 1},
		{"two certificates", append(cert1, cert2...), 2},
		{"empty input", []byte{}, 0},
		{"key block only — no certs extracted", keyOnly, 0},
		{"cert followed by key", append(cert1, keyOnly...), 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := certFromPem(tc.input)
			require.NoError(t, err)
			assert.Len(t, result.Certificate, tc.wantCerts)
		})
	}
}

// --- TestPrivateKeyFromPem ---------------------------------------------------

func TestPrivateKeyFromPem(t *testing.T) {
	tests := []struct {
		name     string
		pemBytes func(t *testing.T) []byte
		password string
		wantErr  bool
	}{
		{
			name:     "unencrypted PKCS8 key with empty password",
			pemBytes: generatePrivateKeyPEM,
			password: "",
		},
		{
			name:     "RFC1423 encrypted key with correct password",
			pemBytes: func(t *testing.T) []byte { return generateEncryptedPrivateKeyPEM(t, "correct") },
			password: "correct",
		},
		{
			name:     "RFC1423 encrypted key with wrong password",
			pemBytes: func(t *testing.T) []byte { return generateEncryptedPrivateKeyPEM(t, "correct") },
			password: "wrong",
			wantErr:  true,
		},
		{
			name:     "no PEM data at all",
			pemBytes: func(_ *testing.T) []byte { return []byte("not-pem-data") },
			password: "",
			wantErr:  true,
		},
		{
			name:     "certificate block only — no key",
			pemBytes: generateSelfSignedCertPEM,
			password: "",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, err := privateKeyFromPem(tc.pemBytes(t), tc.password)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, key)
			_, ok := key.(*ecdsa.PrivateKey)
			assert.True(t, ok, "expected *ecdsa.PrivateKey")
		})
	}
}
