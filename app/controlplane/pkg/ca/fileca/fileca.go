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
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	fulcioca "github.com/sigstore/fulcio/pkg/ca"
	"github.com/sigstore/fulcio/pkg/ca/baseca"
	"github.com/sigstore/fulcio/pkg/identity"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"go.step.sm/crypto/pemutil"
)

const CAName = "fileCA"

type FileCA struct {
	ca fulcioca.CertificateAuthority
}

func New(certPath, keyPath, keyPass string, verify bool) (*FileCA, error) {
	var err error
	baseCA := &baseca.BaseCA{}
	baseCA.SignerWithChain, err = loadKeyPair(certPath, keyPath, keyPass)
	if err != nil {
		return nil, err
	}

	if verify {
		// if the CA is a signer, verify the chain
		chain, signer := baseCA.GetSignerWithChain()
		if err := fulcioca.VerifyCertChain(chain, signer); err != nil {
			return nil, err
		}
	}

	return &FileCA{
		ca: baseCA,
	}, nil
}

func loadKeyPair(certPath, keyPath, keyPass string) (*fulcioca.SignerCerts, error) {
	var (
		certs []*x509.Certificate
		err   error
		key   crypto.Signer
	)

	data, err := os.ReadFile(filepath.Clean(certPath))
	if err != nil {
		return nil, err
	}
	certs, err = cryptoutils.LoadCertificatesFromPEM(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	opaqueKey, err := pemutil.Read(keyPath, pemutil.WithPassword([]byte(keyPass)))
	if err != nil {
		return nil, err
	}

	var ok bool
	key, ok = opaqueKey.(crypto.Signer)
	if !ok {
		return nil, errors.New(`fileca: loaded private key can't be used to sign`)
	}

	return &fulcioca.SignerCerts{Certs: certs, Signer: key}, nil
}

func (f FileCA) CreateCertificateFromCSR(ctx context.Context, principal identity.Principal, csr *x509.CertificateRequest) (*fulcioca.CodeSigningCertificate, error) {
	return f.ca.CreateCertificate(ctx, principal, csr.PublicKey)
}

func (f FileCA) GetRootChain(ctx context.Context) ([]*x509.Certificate, error) {
	tb, err := f.ca.TrustBundle(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load trust bundle: %w", err)
	}
	return tb[0], nil
}

func (f FileCA) GetName() string {
	return CAName
}
