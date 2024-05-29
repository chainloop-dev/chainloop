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

// KeyVal V2 secrets implementation for Hashicorp Vault
package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/docker/distribution/uuid"
	"github.com/go-kratos/kratos/v2/log"
	vault "github.com/hashicorp/vault/api"
)

type Manager struct {
	client       *vault.KVv2
	secretPrefix string
	logger       *log.Helper
}

type NewManagerOpts struct {
	AuthToken, Address, MountPath, SecretPrefix string
	Logger                                      log.Logger
	Role                                        credentials.Role
}

type Role int64

const (
	Reader Role = iota
	Writer
)

const defaultKVMountPath = "secret"
const healthCheckSecret = "chainloop-healthcheck"
const healthCheckNonExisting = "chainloop-non-existing"

// NewManager creates a new credentials manager that uses Hashicorp Vault as backend
// Configured to write secrets in the KVv2 engine referenced by the provided mount path.
// SecretPrefix is used to namespace secrets in the KVv2 engine during write operations.
func NewManager(opts *NewManagerOpts) (*Manager, error) {
	if opts.AuthToken == "" || opts.Address == "" {
		return nil, errors.New("auth token and instance address are required")
	}

	config := vault.DefaultConfig()
	config.Address = opts.Address
	config.Timeout = 1 * time.Second

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, err
	}

	client.SetToken(opts.AuthToken)

	mountPath := defaultKVMountPath
	if opts.MountPath != "" {
		mountPath = opts.MountPath
	}

	l := opts.Logger
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	logger := servicelogger.ScopedHelper(l, "credentials/vault")
	logger.Infow("msg", "configuring vault", "address", opts.Address, "mount_path", mountPath, "prefix", opts.SecretPrefix, "role", opts.Role)

	// Check address, token validity and mount path
	kv := client.KVv2(mountPath)
	if opts.Role == credentials.RoleReader {
		if err := validateReaderClient(kv, opts.SecretPrefix); err != nil {
			return nil, fmt.Errorf("validating client: %w", err)
		}
	} else {
		if err := validateWriterClient(kv, opts.SecretPrefix); err != nil {
			return nil, fmt.Errorf("validating client: %w", err)
		}
	}

	return &Manager{kv, opts.SecretPrefix, logger}, nil
}

// validateWriterClient checks if the client is valid by writing and deleting a secret
// in the provided mount path.
func validateWriterClient(kv *vault.KVv2, pathPrefix string) error {
	ctx := context.Background()
	keyPath := strings.Join([]string{pathPrefix, healthCheckSecret}, "/")
	if _, err := kv.Put(ctx, keyPath, nil); err != nil {
		return err
	}

	if err := kv.DeleteMetadata(ctx, keyPath); err != nil {
		return fmt.Errorf("deleting health check secret: %w", err)
	}

	return nil
}

func validateReaderClient(kv *vault.KVv2, pathPrefix string) error {
	ctx := context.Background()
	// try to retrieve a non-existing key
	// if we get 404 means that we have permissions to read in that path
	keyPath := strings.Join([]string{pathPrefix, healthCheckNonExisting}, "/")
	_, err := kv.Get(ctx, keyPath)
	if err != nil {
		if errors.Is(err, vault.ErrSecretNotFound) {
			// Everything is ok
			return nil
		}

		return err
	}

	return nil
}

func (m *Manager) SaveCredentials(ctx context.Context, orgID string, creds any) (string, error) {
	credsM, err := structToMap(creds)
	if err != nil {
		return "", fmt.Errorf("converting struct to map: %w", err)
	}

	secretName := strings.Join([]string{m.secretPrefix, orgID, uuid.Generate().String()}, "/")
	m.logger.Infow("msg", "storing credentials", "path", secretName)

	_, err = m.client.Put(ctx, secretName, credsM)
	if err != nil {
		return "", fmt.Errorf("creating secret in Vault: %w", err)
	}

	return secretName, nil
}

func (m *Manager) ReadCredentials(ctx context.Context, secretID string, creds any) error {
	m.logger.Infow("msg", "reading credentials", "path", secretID)

	s, err := m.client.Get(ctx, secretID)
	if err != nil {
		if errors.Is(err, vault.ErrSecretNotFound) {
			return fmt.Errorf("%w: path=%s", credentials.ErrNotFound, secretID)
		}

		return fmt.Errorf("reading secret from Vault: %w", err)
	}

	if err := mapToStruct(s.Data, creds); err != nil {
		return fmt.Errorf("converting secret to struct: %w", err)
	}

	return nil
}

func (m *Manager) DeleteCredentials(ctx context.Context, secretID string) error {
	m.logger.Infow("msg", "deleting credentials", "path", secretID)
	return m.client.DeleteMetadata(ctx, secretID)
}

// convert from struct to map[string]interface{}
func structToMap(i interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	err = json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func mapToStruct(i map[string]interface{}, o interface{}) error {
	b, err := json.Marshal(i)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, o)
	if err != nil {
		return err
	}

	return nil
}

func (r Role) String() string {
	switch r {
	case Reader:
		return "reader"
	case Writer:
		return "writer"
	}
	return "unknown"
}
