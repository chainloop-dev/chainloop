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

package fileca

import (
	"context"
	"crypto/x509"
	"fmt"

	fulcioca "github.com/sigstore/fulcio/pkg/ca"
	fulciofileca "github.com/sigstore/fulcio/pkg/ca/fileca"
	"github.com/sigstore/fulcio/pkg/identity"
)

type FileCA struct {
	ca fulcioca.CertificateAuthority
}

func New(certPath, keyPath, keyPass string, watch bool) (*FileCA, error) {
	wrappedCa, err := fulciofileca.NewFileCA(certPath, keyPath, keyPass, watch)
	if err != nil {
		return nil, fmt.Errorf("failed to create file CA: %w", err)
	}
	return &FileCA{
		ca: wrappedCa,
	}, nil
}

func (f FileCA) CreateCertificateFromCSR(ctx context.Context, principal identity.Principal, csr *x509.CertificateRequest) (*fulcioca.CodeSigningCertificate, error) {
	return f.ca.CreateCertificate(ctx, principal, csr.PublicKey)
}
