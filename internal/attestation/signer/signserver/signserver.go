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
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	sigstoresigner "github.com/sigstore/sigstore/pkg/signature"
)

// ReferenceScheme is the scheme to be used when using signserver keys. Ex: signserver://host/worker
const ReferenceScheme = "signserver"

// Signer implements a signer for SignServer
type Signer struct {
	host, worker string
}

var _ sigstoresigner.Signer = (*Signer)(nil)

func NewSigner(host, worker string) *Signer {
	return &Signer{
		host:   host,
		worker: worker,
	}
}

func (s Signer) PublicKey(opts ...sigstoresigner.PublicKeyOption) (crypto.PublicKey, error) {
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
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
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
