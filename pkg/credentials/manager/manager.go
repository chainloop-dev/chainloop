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

package manager

import (
	"errors"
	"fmt"
	"io"

	"github.com/bufbuild/protovalidate-go"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	api "github.com/chainloop-dev/chainloop/pkg/credentials/api/credentials/v1"
	"github.com/chainloop-dev/chainloop/pkg/credentials/aws"
	"github.com/chainloop-dev/chainloop/pkg/credentials/azurekv"
	"github.com/chainloop-dev/chainloop/pkg/credentials/gcp"
	"github.com/chainloop-dev/chainloop/pkg/credentials/vault"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func NewFromConfig(conf *api.Credentials, role credentials.Role, l log.Logger) (credentials.ReaderWriter, error) {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	if awsc := conf.GetAwsSecretManager(); awsc != nil {
		return newAWSCredentialsManager(awsc, conf.SecretPrefix, role, l)
	}

	if gcpc := conf.GetGcpSecretManager(); gcpc != nil {
		return newGCPCredentialsManager(gcpc, conf.SecretPrefix, role, l)
	}

	if vaultc := conf.GetVault(); vaultc != nil {
		return newVaultCredentialsManager(vaultc, conf.SecretPrefix, role, l)
	}

	if creds := conf.GetAzureKeyVault(); creds != nil {
		return newAzureKBManager(creds, conf.SecretPrefix, role, l)
	}

	return nil, errors.New("no credentials manager configuration found")
}

func validateConfig(msg protoreflect.ProtoMessage) error {
	validator, err := protovalidate.New()
	if err != nil {
		return fmt.Errorf("creating validator: %w", err)
	}

	return validator.Validate(msg)
}

func newAzureKBManager(conf *api.Credentials_AzureKeyVault, prefix string, r credentials.Role, l log.Logger) (*azurekv.Manager, error) {
	if err := validateConfig(conf); err != nil {
		return nil, fmt.Errorf("uncompleted configuration for Azure Key Vault: %w", err)
	}

	opts := &azurekv.NewManagerOpts{
		TenantID:     conf.GetTenantId(),
		ClientID:     conf.GetClientId(),
		ClientSecret: conf.GetClientSecret(),
		VaultURI:     conf.GetVaultUri(),
		Logger:       l,
		SecretPrefix: prefix,
		Role:         r,
	}

	m, err := azurekv.NewManager(opts)
	if err != nil {
		return nil, fmt.Errorf("configuring the secrets manager: %w", err)
	}

	if opts.Role == credentials.RoleReader {
		if err := azurekv.ValidateReaderClient(m, prefix); err != nil {
			return nil, fmt.Errorf("validating Azure KeyVault reader client: %w", err)
		}
	} else {
		if err := azurekv.ValidateWriterClient(m, prefix); err != nil {
			return nil, fmt.Errorf("validating Azure KeyVault writer client: %w", err)
		}
	}

	return m, nil
}

func newAWSCredentialsManager(conf *api.Credentials_AWSSecretManager, prefix string, r credentials.Role, l log.Logger) (*aws.Manager, error) {
	if err := validateConfig(conf); err != nil {
		return nil, fmt.Errorf("uncompleted configuration for AWS secret manager: %w", err)
	}

	opts := &aws.NewManagerOpts{
		Region:    conf.Region,
		AccessKey: conf.GetCreds().GetAccessKey(), SecretKey: conf.GetCreds().GetSecretKey(),
		Logger:       l,
		SecretPrefix: prefix,
		Role:         r,
	}

	m, err := aws.NewManager(opts)
	if err != nil {
		return nil, fmt.Errorf("configuring the secrets manager: %w", err)
	}

	return m, nil
}

func newVaultCredentialsManager(conf *api.Credentials_Vault, prefix string, r credentials.Role, l log.Logger) (*vault.Manager, error) {
	if err := validateConfig(conf); err != nil {
		return nil, fmt.Errorf("uncompleted configuration for Vault secret manager: %w", err)
	}

	opts := &vault.NewManagerOpts{
		AuthToken: conf.Token, Address: conf.Address,
		MountPath: conf.MountPath, Logger: l,
		SecretPrefix: prefix,
		Role:         r,
	}

	m, err := vault.NewManager(opts)
	if err != nil {
		return nil, fmt.Errorf("configuring vault: %w", err)
	}

	return m, nil
}

func newGCPCredentialsManager(conf *api.Credentials_GCPSecretManager, prefix string, r credentials.Role, l log.Logger) (*gcp.Manager, error) {
	if err := validateConfig(conf); err != nil {
		return nil, fmt.Errorf("uncompleted configuration for GCP secret manager: %w", err)
	}

	opts := &gcp.NewManagerOpts{
		ProjectID:         conf.ProjectId,
		ServiceAccountKey: conf.ServiceAccountKey,
		Logger:            l,
		SecretPrefix:      prefix,
		Role:              r,
	}

	m, err := gcp.NewManager(opts)
	if err != nil {
		return nil, fmt.Errorf("configuring the GCP secret manager: %w", err)
	}

	return m, nil
}
