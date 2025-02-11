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

package ca

import (
	"context"
	"crypto/x509"
	"fmt"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/ca/ejbca"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/ca/fileca"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/sigstore/fulcio/pkg/ca"
	"github.com/sigstore/fulcio/pkg/identity"
)

type CertificateAuthority interface {
	// CreateCertificateFromCSR accepts a Certificate Request and generates a certificate signed by a signing authority
	CreateCertificateFromCSR(ctx context.Context, principal identity.Principal, csr *x509.CertificateRequest) (*ca.CodeSigningCertificate, error)
	GetRootChain(ctx context.Context) ([]*x509.Certificate, error)
}

type CertificateAuthorities struct {
	CAs      []CertificateAuthority
	SignerCA CertificateAuthority
}

func NewCertificateAuthoritiesFromConfig(configCAs []*conf.CA, logger log.Logger) (*CertificateAuthorities, error) {
	var (
		err         error
		authorities []CertificateAuthority
		issuerCA    CertificateAuthority
	)

	for _, configCA := range configCAs {
		var authority CertificateAuthority
		if configCA.GetFileCa() != nil {
			fileCa := configCA.GetFileCa()
			_ = logger.Log(log.LevelInfo, "msg", "Keyless: File CA configured")
			authority, err = fileca.New(fileCa.GetCertPath(), fileCa.GetKeyPath(), fileCa.GetKeyPass(), false)
		}

		if configCA.GetEjbcaCa() != nil {
			ejbcaCa := configCA.GetEjbcaCa()
			_ = logger.Log(log.LevelInfo, "msg", "Keyless: EJBCA CA configured")
			authority, err = ejbca.New(ejbcaCa.GetServerUrl(), ejbcaCa.GetKeyPath(), ejbcaCa.GetCertPath(), ejbcaCa.GetRootCaPath(), ejbcaCa.GetCertificateProfileName(), ejbcaCa.GetEndEntityProfileName(), ejbcaCa.GetCertificateAuthorityName())
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create file CA: %w", err)
		}
		if authority != nil {
			authorities = append(authorities, authority)
			if configCA.Issuer {
				issuerCA = authority
			}
		}
	}

	// If there are more than 1 authority, the `issuer` property must be set
	if len(authorities) > 1 && issuerCA == nil {
		return nil, fmt.Errorf("at least one issuer CA needs to be configured")
	}

	return &CertificateAuthorities{
		CAs:      authorities, // it might be empty
		SignerCA: issuerCA,
	}, nil
}

func (c *CertificateAuthorities) GetAuthorities() []CertificateAuthority {
	return c.CAs
}

func (c *CertificateAuthorities) GetSignerCA() (CertificateAuthority, error) {
	if c.SignerCA != nil {
		return c.SignerCA, nil
	}

	// use as signer if only one is available
	if len(c.CAs) == 1 {
		return c.CAs[0], nil
	}

	return nil, fmt.Errorf("no signer CA found")
}
