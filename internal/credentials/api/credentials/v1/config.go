//
// Copyright 2023 The Chainloop Authors.
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

package v1

import (
	"errors"
	"fmt"
	"io"

	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/internal/credentials/aws"
	"github.com/chainloop-dev/chainloop/internal/credentials/gcp"
	"github.com/chainloop-dev/chainloop/internal/credentials/vault"
	"github.com/go-kratos/kratos/v2/log"
)

func NewFromConfig(conf *Credentials, l log.Logger) (credentials.ReaderWriter, error) {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	if awsc := conf.GetAwsSecretManager(); awsc != nil {
		return newAWSCredentialsManager(awsc, l)
	}

	if gcpc := conf.GetGcpSecretManager(); gcpc != nil {
		return newGCPCredentialsManager(gcpc, l)
	}

	if vaultc := conf.GetVault(); vaultc != nil {
		return newVaultCredentialsManager(vaultc, l)
	}

	return nil, errors.New("no credentials manager configuration found")
}

func newAWSCredentialsManager(conf *Credentials_AWSSecretManager, l log.Logger) (*aws.Manager, error) {
	if err := conf.ValidateAll(); err != nil {
		return nil, fmt.Errorf("uncompleted configuration for AWS secret manager: %w", err)
	}

	opts := &aws.NewManagerOpts{
		Region:    conf.Region,
		AccessKey: conf.GetCreds().GetAccessKey(), SecretKey: conf.GetCreds().GetSecretKey(),
		Logger: l,
	}

	m, err := aws.NewManager(opts)
	if err != nil {
		return nil, fmt.Errorf("configuring the secrets manager: %w", err)
	}

	_ = l.Log(log.LevelInfo, "msg", "secrets manager configured", "backend", "AWS secret manager")

	return m, nil
}

func newVaultCredentialsManager(conf *Credentials_Vault, l log.Logger) (*vault.Manager, error) {
	if err := conf.ValidateAll(); err != nil {
		return nil, fmt.Errorf("uncompleted configuration for Vault secret manager: %w", err)
	}

	opts := &vault.NewManagerOpts{
		AuthToken: conf.Token, Address: conf.Address,
		MountPath: conf.MountPath, Logger: l,
	}

	m, err := vault.NewManager(opts)
	if err != nil {
		return nil, fmt.Errorf("configuring vault: %w", err)
	}

	_ = l.Log(log.LevelInfo, "msg", "secrets manager configured", "backend", "Vault")

	return m, nil
}

func newGCPCredentialsManager(conf *Credentials_GCPSecretManager, l log.Logger) (*gcp.Manager, error) {
	if err := conf.ValidateAll(); err != nil {
		return nil, fmt.Errorf("uncompleted configuration for GCP secret manager: %w", err)
	}

	opts := &gcp.NewManagerOpts{
		ProjectID:         conf.ProjectId,
		ServiceAccountKey: conf.ServiceAccountKey,
		Logger:            l,
	}

	m, err := gcp.NewManager(opts)
	if err != nil {
		return nil, fmt.Errorf("configuring the GCP secret manager: %w", err)
	}

	_ = l.Log(log.LevelInfo, "msg", "secrets manager configured", "backend", "GCP secret manager")

	return m, nil
}
