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

package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/smithy-go"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/google/uuid"

	"github.com/go-kratos/kratos/v2/log"
)

type SecretsManagerIface interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	DeleteSecret(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error)
}

type Manager struct {
	client       SecretsManagerIface
	secretPrefix string
	logger       *log.Helper
}

type NewManagerOpts struct {
	Region, AccessKey, SecretKey, SecretPrefix string
	Logger                                     log.Logger
	Role                                       credentials.Role
}

func NewManager(opts *NewManagerOpts) (*Manager, error) {
	if opts.Region == "" || opts.AccessKey == "" || opts.SecretKey == "" {
		return nil, errors.New("region, accessKey and the secretKey are required")
	}

	l := opts.Logger
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	logger := servicelogger.ScopedHelper(l, "credentials/aws-secrets-manager")
	logger.Infow("msg", "configuring secrets-manager", "region", opts.Region, "role", opts.Role, "prefix", opts.SecretPrefix)

	config, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(opts.Region),
		config.WithCredentialsProvider(
			awscreds.NewStaticCredentialsProvider(opts.AccessKey, opts.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	return &Manager{
		client:       secretsmanager.NewFromConfig(config),
		secretPrefix: opts.SecretPrefix, logger: logger,
	}, nil
}

// Save Credentials, this is a generic function that can be used to save any type of credentials
// as long as they can be passed to json.Marshal
func (m *Manager) SaveCredentials(ctx context.Context, orgID string, creds any) (string, error) {
	secretName := strings.Join([]string{m.secretPrefix, orgID, uuid.New().String()}, "/")

	// Store the credentials as json key pairs
	c, err := json.Marshal(creds)
	if err != nil {
		return "", fmt.Errorf("marshaling credentials to be stored: %w", err)
	}

	if _, err = m.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name: aws.String(secretName), SecretString: aws.String(string(c)),
	}); err != nil {
		return "", fmt.Errorf("creating secret in AWS: %w", err)
	}

	return secretName, nil
}

func (m *Manager) ReadCredentials(ctx context.Context, secretID string, creds any) error {
	resp, err := m.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretID),
	})

	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case (&types.ResourceNotFoundException{}).ErrorCode():
				return fmt.Errorf("%w: path=%s", credentials.ErrNotFound, secretID)
			default:
				return fmt.Errorf("getting AWS Secret Value: %w", err)
			}
		}

		return fmt.Errorf("getting AWS Secret Value: %w", err)
	}

	return json.Unmarshal([]byte(*resp.SecretString), creds)
}

func (m *Manager) DeleteCredentials(ctx context.Context, secretID string) error {
	_, err := m.client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId: aws.String(secretID),
	})

	return err
}
