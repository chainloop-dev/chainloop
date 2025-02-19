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

package biz

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

type TimestampAuthority struct {
	Issuer    bool
	URL       *url.URL
	CertChain []*x509.Certificate
}

type TimestampAuthorityUseCase struct {
	authorities []*TimestampAuthority
	logger      *log.Helper
}

func NewTimestampAuthorityUseCase(config *conf.Bootstrap, l log.Logger) (*TimestampAuthorityUseCase, error) {
	var issuerFound bool
	auths := make([]*TimestampAuthority, 0)
	for _, tsaConf := range config.GetTimestampAuthorities() {
		tsa, err := parseTSA(tsaConf)
		if err != nil {
			return nil, err
		}
		if issuerFound && tsa.Issuer {
			return nil, fmt.Errorf("duplicate timestamp issuer in tsa config")
		}
		issuerFound = tsa.Issuer
		auths = append(auths, tsa)
	}
	if len(auths) > 0 && !issuerFound {
		return nil, fmt.Errorf("timestamp issuer not found in tsa config")
	}

	logger := servicelogger.ScopedHelper(l, "biz/timestamp")
	logger.Info(fmt.Sprintf("Timestamp authority configured with %d TSA servers", len(auths)))

	return &TimestampAuthorityUseCase{authorities: auths, logger: logger}, nil
}

func parseTSA(tsaConf *conf.TSA) (*TimestampAuthority, error) {
	tsa := &TimestampAuthority{}
	if tsaConf.Issuer {
		// only require URL if it's the main one, as others will be used for verification only
		tsaUrl, err := url.Parse(tsaConf.Url)
		if err != nil {
			return nil, fmt.Errorf("failed to parse TSA URL: %w", err)
		}
		tsa.URL = tsaUrl
	}
	tsa.Issuer = tsaConf.Issuer
	if tsaConf.GetCertChainPath() == "" {
		return nil, fmt.Errorf("missing certificate path for TSA")
	}
	pemBytes, err := os.ReadFile(tsaConf.GetCertChainPath())
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate chain: %w", err)
	}
	certs, err := cryptoutils.LoadCertificatesFromPEM(bytes.NewReader(pemBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to load certificates: %w", err)
	}
	tsa.CertChain = certs

	return tsa, nil
}

func (uc *TimestampAuthorityUseCase) GetCurrentTSA() *TimestampAuthority {
	for _, tsa := range uc.authorities {
		if tsa.Issuer {
			return tsa
		}
	}
	// Nil means not configured and needs to be handled correctly
	return nil
}
