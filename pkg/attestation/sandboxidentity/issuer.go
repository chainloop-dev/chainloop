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
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

// Issuer issues identity tokens. The platform uses it each time it spawns a sandbox, with a
// client authenticated as a caller that holds the dedicated permission.
type Issuer struct {
	client pb.SigningServiceClient
}

// NewIssuer returns an issuer that gets its certificates from the signing service of the control plane.
func NewIssuer(client pb.SigningServiceClient) *Issuer {
	return &Issuer{client: client}
}

// Issue makes a new key, gets a certificate with the mark for it, and signs the claims. It sets
// the time of signing. The key is discarded after it signs, so it signs only this identity.
func (i *Issuer) Issue(ctx context.Context, claims *Claims) (string, error) {
	if claims == nil {
		return "", errors.New("claims are required")
	}

	// Work on a copy, the time of signing is set below
	toSign := *claims
	toSign.IssuedAt = jwt.NewNumericDate(time.Now())
	if err := toSign.validate(); err != nil {
		return "", err
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("generating a key: %w", err)
	}

	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: uuid.NewString()}}, key)
	if err != nil {
		return "", fmt.Errorf("generating a certificate request: %w", err)
	}

	resp, err := i.client.GenerateSigningCert(ctx, &pb.GenerateSigningCertRequest{
		CertificateSigningRequest: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr}),
		SandboxIdentityIssuer:     true,
	})
	if err != nil {
		return "", fmt.Errorf("getting a certificate: %w", err)
	}

	chain, err := cryptoutils.UnmarshalCertificatesFromPEM([]byte(strings.Join(resp.GetChain().GetCertificates(), "\n")))
	if err != nil {
		return "", fmt.Errorf("parsing the certificate chain: %w", err)
	}

	// A control plane without support for the mark ignores the request
	if len(chain) == 0 || !HasMark(chain[0]) {
		return "", errors.New("the control plane did not mark the certificate for a sandbox identity")
	}

	toSign.IssuedAt = jwt.NewNumericDate(time.Now())

	return Sign(key, chain, &toSign)
}
