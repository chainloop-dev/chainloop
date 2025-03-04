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

package signserver

import (
	"bytes"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"

	sigstoresigner "github.com/sigstore/sigstore/pkg/signature"
	"github.com/youmark/pkcs8"
)

// ReferenceScheme is the scheme to be used when using signserver keys. Ex: signserver://host/worker
const ReferenceScheme = "signserver"

// Signer implements a signer for SignServer
type Signer struct {
	host, worker, caPath, clientCertPath, clientCertPass string
}

type SignerOpt func(*Signer)

func WithCAPath(caPath string) SignerOpt {
	return func(s *Signer) {
		s.caPath = caPath
	}
}

func WithClientCertPath(clientCertPath string) SignerOpt {
	return func(s *Signer) {
		s.clientCertPath = clientCertPath
	}
}
func WithClientCertPass(clientCertPass string) SignerOpt {
	return func(s *Signer) {
		s.clientCertPass = clientCertPass
	}
}

var _ sigstoresigner.Signer = (*Signer)(nil)

func NewSigner(host, worker string, opts ...SignerOpt) *Signer {
	s := &Signer{
		host:   host,
		worker: worker,
	}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s Signer) PublicKey(_ ...sigstoresigner.PublicKeyOption) (crypto.PublicKey, error) {
	return nil, errors.New("public key not yet supported for SignServer")
}

// SignMessage Signs a message by calling to SignServer
func (s Signer) SignMessage(message io.Reader, _ ...sigstoresigner.SignOption) ([]byte, error) {
	url, err := url.Parse(fmt.Sprintf("https://%s/signserver/process", s.host))
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	body := &bytes.Buffer{}
	mpwriter := multipart.NewWriter(body)

	// Send worker name
	err = mpwriter.WriteField("workerName", s.worker)
	if err != nil {
		return nil, fmt.Errorf("failed to write field: %w", err)
	}

	// Send payload
	part, err := mpwriter.CreateFormFile("file", "attestation.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create form field: %w", err)
	}
	_, err = io.Copy(part, message)
	if err != nil {
		return nil, fmt.Errorf("failed to copy message: %w", err)
	}
	err = mpwriter.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequest("POST", url.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Content-Type", mpwriter.FormDataContentType())
	client := &http.Client{}

	var caPool *x509.CertPool
	var tlsConfig *tls.Config
	if s.caPath != "" {
		caPool = x509.NewCertPool()
		caContents, err := os.ReadFile(s.caPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read ca cert: %w", err)
		}
		caPool.AppendCertsFromPEM(caContents)
		tlsConfig = &tls.Config{RootCAs: caPool, MinVersion: tls.VersionTLS12}
	}

	if s.clientCertPath != "" {
		var cert tls.Certificate
		pemBytes, err := os.ReadFile(s.clientCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read client cert file: %w", err)
		}
		if s.clientCertPass != "" {
			cert, err = certFromPem(pemBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to read client cert file: %w", err)
			}
			// decrypt the private key
			key, err := privateKeyFromPem(pemBytes, s.clientCertPass)
			if err != nil {
				return nil, fmt.Errorf("failed to load private key: %w", err)
			}
			cert.PrivateKey = key
		} else {
			// Try with unencrypted PEM
			cert, err = tls.LoadX509KeyPair(s.clientCertPath, s.clientCertPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load client cert and key: %w", err)
			}
		}

		if tlsConfig == nil {
			tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if tlsConfig != nil {
		client.Transport = &http.Transport{TLSClientConfig: tlsConfig}
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to send request: status %d", res.StatusCode)
	}

	resBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return resBytes, nil
}

// ParseKeyReference interprets a key reference for SignServer
func ParseKeyReference(keyPath string) (string, string, error) {
	parts := strings.SplitAfter(keyPath, "://")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid key path: %s", keyPath)
	}
	parts = strings.Split(parts[1], "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid key path: %s", keyPath)
	}
	return parts[0], parts[1], nil
}

func certFromPem(pemBytes []byte) (tls.Certificate, error) {
	var cert tls.Certificate
	for {
		var certDERBlock *pem.Block
		certDERBlock, pemBytes = pem.Decode(pemBytes)
		if certDERBlock == nil {
			break
		}
		if certDERBlock.Type == "CERTIFICATE" {
			cert.Certificate = append(cert.Certificate, certDERBlock.Bytes)
		}
	}
	return cert, nil
}

func privateKeyFromPem(pemBytes []byte, password string) (any, error) {
	var keyDERBlock *pem.Block
	for {
		keyDERBlock, pemBytes = pem.Decode(pemBytes)
		if keyDERBlock == nil {
			return nil, errors.New("failed to find any PEM data in key input")
		}

		if keyDERBlock.Type == "PRIVATE KEY" || strings.HasSuffix(keyDERBlock.Type, " PRIVATE KEY") {
			break
		}
	}

	// Try with RFC1423
	var key any
	derBytes, err := x509.DecryptPEMBlock(keyDERBlock, []byte(password))
	if err != nil {
		// try with PKCS#8 encryption (not supported by stdlib)
		key, err = pkcs8.ParsePKCS8PrivateKey(keyDERBlock.Bytes, []byte(password))
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt private key: %w", err)
		}
	} else {
		key, err = x509.ParsePKCS8PrivateKey(derBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	return key, nil
}
