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

package ejbca

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/ca"
	fulcioca "github.com/sigstore/fulcio/pkg/ca"
	"github.com/sigstore/fulcio/pkg/identity"
)

type EJBCA struct {
	// Connection settings
	// EJBCA installation. ie https://localhost/ejbca/
	serverURL string
	// client private key used for authentication
	keyPath string
	// client certificate used for authentication
	certPath string
	// root CA
	rootCAPath string

	// Entity Configuration settings
	// https://docs.keyfactor.com/signserver/latest/tutorial-signserver-container-signing-with-cosign#id-(6.3latest)Tutorial-SignServerContainerSigningwithCosign-Step2-Issuesigningcertificate
	certificateProfileName string
	endEntityProfileName   string
	caName                 string
}

var _ ca.CertificateAuthority = (*EJBCA)(nil)

func New(serverURL, keyPath, certPath, rootCAPath, certProfileName, endEntityProfileName, caName string) *EJBCA {
	return &EJBCA{
		serverURL:              serverURL,
		keyPath:                keyPath,
		certPath:               certPath,
		rootCAPath:             rootCAPath,
		certificateProfileName: certProfileName,
		endEntityProfileName:   endEntityProfileName,
		caName:                 caName,
	}
}

// Request represents an EJBCA enrollment request (/v1/certificate/pkcs10enroll)
type Request struct {
	CertificateRequest       string `json:"certificate_request,omitempty"`
	CertificateProfileName   string `json:"certificate_profile_name,omitempty"`
	EndEntityProfileName     string `json:"end_entity_profile_name,omitempty"`
	CertificateAuthorityName string `json:"certificate_authority_name,omitempty"`
	Username                 string `json:"username,omitempty"`
	IncludeChain             bool   `json:"include_chain,omitempty"`
}

// Response represents an enrollment response
type Response struct {
	Certificate      string   `json:"certificate,omitempty"`
	SerialNumber     string   `json:"serial_number,omitempty"`
	ResponseFormat   string   `json:"response_format,omitempty"`
	CertificateChain []string `json:"certificate_chain,omitempty"`
}

func (e EJBCA) CreateCertificateFromCSR(ctx context.Context, principal identity.Principal, csr *x509.CertificateRequest) (*fulcioca.CodeSigningCertificate, error) {
	ejbcaReq := &Request{
		CertificateRequest:       string(csr.Raw),
		CertificateProfileName:   e.certificateProfileName,
		EndEntityProfileName:     e.endEntityProfileName,
		CertificateAuthorityName: e.caName,
		Username:                 principal.Name(ctx),
		IncludeChain:             true,
	}
	ejbcaReqBytes, err := json.Marshal(ejbcaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal EJBCA Request: %w", err)
	}

	cert, err := tls.LoadX509KeyPair(e.certPath, e.keyPath)
	if err != nil {
		return nil, fmt.Errorf("error loading X509 key pair: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	if e.rootCAPath != "" {
		caCert, err := os.ReadFile(e.rootCAPath)
		if err != nil {
			return nil, fmt.Errorf("error reading root CA: %w", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: tlsConfig,
	}}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/ejbca-rest-api/v1/certificate/pkcs10enroll", e.serverURL), bytes.NewReader(ejbcaReqBytes))
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to send request: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("wrong status creating certificate: %v", resp.Status)
	}

	var response Response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %w", err)
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unable to parse response body: %w", err)
	}

	// Decode certificate
	derCert, err := base64.StdEncoding.DecodeString(response.Certificate)
	if err != nil {
		return nil, fmt.Errorf("unable to decode response body: %w", err)
	}

	// Decode chain
	chain := make([]*x509.Certificate, 0)
	for _, c := range response.CertificateChain {
		decodedCert, err := base64.StdEncoding.DecodeString(c)
		if err != nil {
			return nil, fmt.Errorf("unable to decode response body: %w", err)
		}
		cert, err := x509.ParseCertificate(decodedCert)
		if err != nil {
			return nil, fmt.Errorf("unable to parse cert chain: %w", err)
		}
		chain = append(chain, cert)
	}

	return fulcioca.CreateCSCFromDER(derCert, chain)
}
