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

// KeyVal V2 secrets implementation for Hashicorp Vault
package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/credentials"
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
	// KubernetesAuth logs in with the pod's service account token instead of AuthToken. Set exactly one of them.
	KubernetesAuth *KubernetesAuthOpts
}

// KubernetesAuthOpts configures Vault's Kubernetes auth method.
type KubernetesAuthOpts struct {
	// Role is the Vault role bound to the service account.
	Role string
	// MountPath of the auth method; defaults to "kubernetes".
	MountPath string
	// TokenPath is the service account token file; defaults to the in-pod projected token.
	TokenPath string
}

const (
	defaultKubernetesAuthMountPath = "kubernetes"
	defaultServiceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec G101 -- a file path
)

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
	if opts.Address == "" {
		return nil, errors.New("instance address is required")
	}
	if (opts.AuthToken == "") == (opts.KubernetesAuth == nil) {
		return nil, errors.New("exactly one of auth token or kubernetes auth is required")
	}

	config := vault.DefaultConfig()
	config.Address = opts.Address
	config.Timeout = 1 * time.Second

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, err
	}

	l := opts.Logger
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	logger := servicelogger.ScopedHelper(l, "credentials/vault")

	if opts.KubernetesAuth != nil {
		login, err := kubernetesLogin(context.Background(), client, opts.KubernetesAuth)
		if err != nil {
			return nil, fmt.Errorf("logging in with kubernetes auth: %w", err)
		}
		go keepLoggedIn(client, opts.KubernetesAuth, login, logger)
	} else {
		client.SetToken(opts.AuthToken)
	}

	mountPath := defaultKVMountPath
	if opts.MountPath != "" {
		mountPath = opts.MountPath
	}

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

// kubernetesLogin exchanges the service account token for a Vault token and sets it on the client. The token file is
// read on every login: a projected service account token is rotated by the kubelet.
func kubernetesLogin(ctx context.Context, client *vault.Client, opts *KubernetesAuthOpts) (*vault.Secret, error) {
	if opts.Role == "" {
		return nil, errors.New("kubernetes auth role is required")
	}
	mount := opts.MountPath
	if mount == "" {
		mount = defaultKubernetesAuthMountPath
	}
	tokenPath := opts.TokenPath
	if tokenPath == "" {
		tokenPath = defaultServiceAccountTokenPath
	}

	jwt, err := os.ReadFile(tokenPath) // #nosec G304 -- operator-configured path
	if err != nil {
		return nil, fmt.Errorf("reading service account token: %w", err)
	}

	secret, err := client.Logical().WriteWithContext(ctx, fmt.Sprintf("auth/%s/login", strings.Trim(mount, "/")), map[string]any{
		"role": opts.Role,
		"jwt":  strings.TrimSpace(string(jwt)),
	})
	if err != nil {
		return nil, err
	}
	if secret == nil || secret.Auth == nil || secret.Auth.ClientToken == "" {
		return nil, errors.New("kubernetes auth login returned no client token")
	}

	client.SetToken(secret.Auth.ClientToken)
	return secret, nil
}

// keepLoggedIn renews the login's lease for as long as Vault allows, then logs in again. It runs for the life of the
// process, like the manager itself.
func keepLoggedIn(client *vault.Client, opts *KubernetesAuthOpts, login *vault.Secret, logger *log.Helper) {
	for {
		if login != nil && login.Auth != nil && login.Auth.Renewable {
			watcher, err := client.NewLifetimeWatcher(&vault.LifetimeWatcherInput{Secret: login})
			if err != nil {
				logger.Errorw("msg", "vault token lifetime watcher", "error", err)
			} else {
				go watcher.Start()
				for done := false; !done; {
					select {
					case err := <-watcher.DoneCh():
						if err != nil {
							logger.Warnw("msg", "vault token renewal stopped", "error", err)
						}
						done = true
					case <-watcher.RenewCh():
					}
				}
				watcher.Stop()
			}
		} else if login != nil && login.Auth != nil {
			// Not renewable: log in again shortly before the lease ends.
			time.Sleep(time.Duration(login.Auth.LeaseDuration) * time.Second * 9 / 10)
		}

		var err error
		if login, err = kubernetesLogin(context.Background(), client, opts); err != nil {
			logger.Errorw("msg", "vault kubernetes re-login", "error", err)
			login = nil
			time.Sleep(10 * time.Second)
		}
	}
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

func (m *Manager) SaveCredentials(ctx context.Context, orgID string, creds any, opts ...credentials.SaveOption) (string, error) {
	credsM, err := structToMap(creds)
	if err != nil {
		return "", fmt.Errorf("converting struct to map: %w", err)
	}

	o := credentials.ApplySaveOptions(opts...)
	secretName := o.SecretName
	if secretName == "" {
		secretName = strings.Join([]string{m.secretPrefix, orgID, uuid.Generate().String()}, "/")
	}
	m.logger.Infow("msg", "storing credentials", "path", secretName)

	_, err = m.client.Put(ctx, secretName, credsM)
	if err != nil {
		return "", fmt.Errorf("storing secret in Vault: %w", err)
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
