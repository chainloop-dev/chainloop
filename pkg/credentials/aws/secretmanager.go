//
// Copyright 2023-2026 The Chainloop Authors.
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
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/smithy-go"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/google/uuid"

	"github.com/go-kratos/kratos/v2/log"
)

type SecretsManagerIface interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	PutSecretValue(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
	DeleteSecret(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error)
}

type Manager struct {
	client       SecretsManagerIface
	secretPrefix string
	logger       *log.Helper
}

// AuthType selects how the manager authenticates to AWS. The zero value is AuthTypeCredentials, so a configuration
// that does not choose keeps using static keys.
type AuthType int

const (
	// AuthTypeCredentials uses the static AccessKey and SecretKey.
	AuthTypeCredentials AuthType = iota
	// AuthTypeAmbient uses the AWS SDK default credential chain (EKS Pod Identity, IRSA, instance role).
	AuthTypeAmbient
)

type NewManagerOpts struct {
	Region, AccessKey, SecretKey, SecretPrefix string
	AuthType                                   AuthType
	Logger                                     log.Logger
	Role                                       credentials.Role
}

func NewManager(opts *NewManagerOpts) (*Manager, error) {
	if opts.Region == "" {
		return nil, errors.New("region is required")
	}
	// The operator chooses ambient authentication explicitly, so forgotten keys fail here instead of silently
	// falling through to whatever the default chain finds (for example the node's instance role).
	switch opts.AuthType {
	case AuthTypeCredentials:
		if opts.AccessKey == "" || opts.SecretKey == "" {
			return nil, errors.New("accessKey and secretKey are required for the credentials auth type")
		}
	case AuthTypeAmbient:
		if opts.AccessKey != "" || opts.SecretKey != "" {
			return nil, errors.New("accessKey and secretKey must not be set for the ambient auth type")
		}
	default:
		return nil, fmt.Errorf("unknown auth type %d", opts.AuthType)
	}

	l := opts.Logger
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	logger := servicelogger.ScopedHelper(l, "credentials/aws-secrets-manager")
	logger.Infow("msg", "configuring secrets-manager", "region", opts.Region, "role", opts.Role, "prefix", opts.SecretPrefix, "ambient", opts.AuthType == AuthTypeAmbient)

	cfg, err := loadConfig(opts)
	if err != nil {
		return nil, fmt.Errorf("loading AWS configuration: %w", err)
	}

	return &Manager{
		client:       secretsmanager.NewFromConfig(cfg),
		secretPrefix: opts.SecretPrefix, logger: logger,
	}, nil
}

// loadConfig uses only the static keys for AuthTypeCredentials, never falling through to ambient credentials. For
// AuthTypeAmbient it resolves credentials through the SDK's default chain, so no long-lived key has to be stored for
// the control plane or CAS.
func loadConfig(opts *NewManagerOpts) (aws.Config, error) {
	if opts.AuthType == AuthTypeCredentials {
		return aws.Config{
			Region:      opts.Region,
			Credentials: awscreds.NewStaticCredentialsProvider(opts.AccessKey, opts.SecretKey, ""),
		}, nil
	}

	return awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(opts.Region))
}

// SaveCredentials saves credentials. If opts includes WithExistingSecret, upserts at the given path.
func (m *Manager) SaveCredentials(ctx context.Context, orgID string, creds any, opts ...credentials.SaveOption) (string, error) {
	o := credentials.ApplySaveOptions(opts...)
	secretName := o.SecretName
	if secretName == "" {
		secretName = strings.Join([]string{m.secretPrefix, orgID, uuid.New().String()}, "/")
	}

	// Store the credentials as json key pairs
	c, err := json.Marshal(creds)
	if err != nil {
		return "", fmt.Errorf("marshaling credentials to be stored: %w", err)
	}

	if o.SecretName != "" {
		// Upsert: try to update existing secret first
		_, err = m.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
			SecretId:     aws.String(secretName),
			SecretString: aws.String(string(c)),
		})
		var notFoundErr *smtypes.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			// Secret does not exist yet, create it
			_, err = m.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
				Name:         aws.String(secretName),
				SecretString: aws.String(string(c)),
			})
		}
	} else {
		_, err = m.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(secretName),
			SecretString: aws.String(string(c)),
		})
	}

	if err != nil {
		return "", fmt.Errorf("saving secret in AWS: %w", err)
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
